// Package progress provides status updates to the terminal as units of work are incrementally completed.
package progress

import (
	"context"
	"io"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// scale represents 100% as a large fixed-point integer to support high-precision fractional updates.
//
// The choice of 1e15 balances high-precision fractional shares in the context
// of, e.g., deep recursion, with sufficient uint64 headroom to prevent overflow
// when performing intermediate percentage calculations (newCurrent * 10000).
//
// Precision starts to degrade as the total number of work units approaches scale,
// but even at this limit, each unit of work represents at least 1 unit of scale.
const scale uint64 = 1e15

// Option defines a functional configuration for Progress.
type Option func(*Progress) // exported to allow callers to create []*progress.Option to pass to New(...)

// WithTracker allows callers to override the default progress.Standard status tracker, e.g.:
//
//   progress.New(ctx, 100, os.Stderr, progress.WithTracker(progress.Fraction))
func WithTracker(s strategy) Option {
	return func(p *Progress) { p.tracker = getTracker(s, p.total.Load()) }
}

// WithTheme allows callers to override the default color (green) of the progress bar, e.g.:
//
//   progress.New(ctx, 100, os.Stderr, progress.WithTheme("yellow"))
//
// Silently falls back to the default ("green") if an invalid theme name is specified.
func WithTheme(c string) Option {
	return func(p *Progress) { p.theme = themeOrDefault(c) }
}

// Progress implements a throttled, concurrency-safe,
// high-precision status indicator for workloads.
type Progress struct {
	// shared state (atomic)
	tracker        statusTracker          // tracks the current progress status
	total          atomic.Uint64          // total work units; 0 for fractional path allocation; > 0 for weight-based accumulation
	current        atomic.Uint64          // accumulates shares of scale as work is completed
	state          atomic.Uint32          // bit-packed word: upper 16 bits for terminal width; lower 16 bits for progress percentage significant digits
	lastState      atomic.Uint32          // previous snapshot of state: used to detect terminal width or progress changes, and skip redundant redraws
	lastStatusVal  atomic.Value           // stores the last Report()ed status (i.e., the result of the last tracker.load())
	lastFrame      atomic.Pointer[string] // stores the last rendered frame string (test observability channel hook)
	bufPool        *bufPool               // provisions reusable / recyclable rendering buffers from a sync.Pool-managed pool

	// configuration (read-only after construction)
	output        io.Writer      // destination writer for the terminal-formatted work progress status updates (nominally os.Stderr)
	layout        layout         // terminal-aware layout state copy used for rendering progress status
	stopChan      chan struct{}  // signals the background rendering loop to perform final cleanup
	doneChan      chan struct{}  // doneChan is closed once the rendering loop has finished its final draw and cursor restoration
	drawNotify    chan struct{}  // used in tests to signal the completion of a draw cycle
	resizeChan    chan os.Signal // handles terminal window resizing events via the syscall.SIGWINCH signal
	resizeHandler resizeHandler  // handles terminal resize events (enables dependency injection in tests)
	closeOnce     sync.Once      // closeOnce ensures that cursor restoration and cleanup logic are executed only once
	clock         clock          // provides the timing source for throttled UI updates, allowing for fake clocks in tests
	isTerminal    func(any) bool // facilitates dependency injection for tests
	theme         *theme         // progress bar color theme
}

// New initializes a throttled, concurrency-safe, high-precision work progress
// tracker and starts a work completion status rendering loop in the background.
//
// The value of the totalUnits parameter determines the accumulation mode used internally:
//
//   - Pass totalUnits >  0 for weight-based accumulation  (when totalUnits is known).
//   - Pass totalUnits == 0 for fractional path allocation (when totalUnits is unknown).
func New(ctx context.Context, totalUnits uint64, output io.Writer, opts ...Option) *Progress {
	p := &Progress{
		tracker:        getTracker(Standard, totalUnits),
		output:         output,
		stopChan:       make(chan struct{}),
		doneChan:       make(chan struct{}),
		resizeChan:     make(chan os.Signal, 1),
		clock:          &realClock{dur: 16 * time.Millisecond},
		theme:          themeOrDefault("green"),
	}

	p.isTerminal = func(v any) bool {
		isTerminal  := isTerminal(v)
		p.isTerminal = func(any) bool { return isTerminal }
		return isTerminal
	}

	p.resizeHandler = p.getResizedTermWidth
	termWidth      := getTermWidth(p.output)

	p.total.Store(min(totalUnits, scale)) // fall back to scale if totalUnits exceeds max precision
	p.state.Store(uint32(termWidth & 0xFFFF) << 16)

	for _, opt := range opts { opt(p) } // allows callers to override defaults via exported Options

	p.initBufPool()
	p.prepareTerminal()

	signal.Notify(p.resizeChan, syscall.SIGWINCH) // listen for a SIGWINCH signal to handle the terminal window being resized

	if p.isTerminal(p.output) { _, _ = io.WriteString(p.output, ansiHideCursor) } // hide the cursor

	go p.renderLoop(ctx)

	return p
}

// InitialBudget returns the full internal scale (100%) to be used
// as the starting budget for tracking fractional progress.
func (p *Progress) InitialBudget() float64 { return float64(scale) }

// AddTotal dynamically increases the total work budget as new tasks are discovered.
// It is concurrency-safe and ensures the total budget never exceeds scale.
func (p *Progress) AddTotal(n uint64) {
	for {
		p.tracker.addTotal(n) // pure no-op call except for fractionTracker; TODO(jeff): clean this up with more a transparent polymorphic implementation
		oldTotal := p.total.Load()
		newTotal := min(oldTotal + n, scale) // fall back to scale if total exceeds max precision
		if p.total.CompareAndSwap(oldTotal, newTotal) { break }
	}
}

// Report updates the current progress status.
//
// The progress calculation depends on the initialization mode:
//
//   - If total >  0: weight represents the relative weight of the work completed,
//     and the progress percentage is calculated as accumulated weight / totalUnits.
//   - If total == 0: weight represents the portion of the InitialBudget(),
//     which must be divided among all sub-tasks by the caller.
func (p *Progress) Report(weight float64, status string) {
	p.tracker.store(uint64(weight), status)

	total := p.total.Load()

	var share uint64
	if total > 0 {
		share = uint64((weight * float64(scale)) / float64(total)) //  weight-based accumulation mode: calculate the share of the total, maintaining precision
	} else {
		share = uint64(weight)                                     // fractional path allocation mode: add the share of the budget directly
	}

	for {
		oldCurrent := p.current.Load()
		oldState   := p.state.Load()

		newCurrent := min(oldCurrent + share, scale) // cap at scale (100%)

		// capture 5 significant digits of the newCurrent value to be stored in the bit-packed
		// p.state field (atomic.Uint32) while avoiding new memory allocations
		//
		// ((newCurrent * 10000 + (scale / 2)) / scale) converts the 1e15 scale
		// to 5 significant digits (1e4) to prevent overflow
		//
		// adding half of the total scale (the divisor) ensures precise
		// rounding to avoid floor truncation during integer division
		//
		// with scale == 1e15 and newCurrent capped at scale, the maximum value of the
		// numerator is ~1e19, which cleanly fits into a uint64 (math.MaxUint64 =~ 1.84e19)

		scaledSigDigits := (newCurrent * 10000 + (scale / 2)) / scale
		oldSigDigits    := oldState & 0xFFFF
		newSigDigits    := uint32(scaledSigDigits & 0xFFFF)
		newState        := (oldState & 0xFFFF0000) | max(newSigDigits, oldSigDigits) // ensure motonicity and preserve terminal width

		if newCurrent == oldCurrent &&
		   newState   == oldState { return } // nothing to do if both current progress and current state already match

		if !p.current.CompareAndSwap(oldCurrent, newCurrent) {
			continue // handle a concurrent Report or AddTotal call
		}

		for !p.state.CompareAndSwap(oldState, newState) { // handle concurrent Report call or terminal resize event
			oldState     = p.state.Load()
			oldSigDigits =  oldState & 0xFFFF
			newState     = (oldState & 0XFFFF0000) | max(newSigDigits, oldSigDigits) // ensure motonicity and preserve terminal width
		}

		break
	}
}

// Close stops the background renderer and waits for cleanup to complete.
func (p *Progress) Close() {
	p.closeOnce.Do(func() {
		close(p.stopChan) // stop the renderLoop goroutine
		<-p.doneChan      // block until renderLoop exits
	})
}

// renderLoop periodically renders progress status updates at ~60 FPS without impeding workers.
func (p *Progress) renderLoop(ctx context.Context) {

	// defer statements are executed in reverse (LIFO) order:
	//
	//   https://go.dev/ref/spec#Defer_statements
	//   https://go.dev/blog/defer-panic-and-recover

	ticker := p.clock.tick()
	defer ticker.Stop()

	defer close(p.doneChan)
	defer signal.Stop(p.resizeChan)

	// the naked []byte buffer is unprotected by design:
	//
	// - goroutine-confined;     owned exclusively by this execution context
	// - synchronously mutated;  loop events process sequentially
	// - lexically-scoped;       lifetime is structurally bound to this function frame
	// - no reference retention; downstream methods never cache or leak the slice pointer

	buf := p.bufPool.get()

	defer func() {
		buf = buf[:0]
		p.bufPool.put(buf)
	}()

	running := true

	for running {

		buf = buf[:0]

		select {
		case <-ctx.Done():
			running = false
		case <-p.stopChan:
			running = false
		default:
		}

		if !running { break }

		select {                 // exit immediately if canceled or explicitly stopped per a Close() call
		case <-ctx.Done():       // parent context canceled, or SIGINIT / SIGTERM / SIGHUP received
			running = false
		case <-p.stopChan:       // Close() called
			running = false
		case <-ticker.ch():
			buf = p.sync(buf)
		case <-p.resizeChan:
			buf = p.handleResize(buf)
		}

		if running {
			select { // post-loop drain: ensure any final pending tick is flushed before deferred cleanup routines execute
			case <-ticker.ch():
				buf = p.sync(buf)
			default:
			}
		}

//		default:
//			select {             // process events normally while running
//			case <-ctx.Done():   // parent context canceled, or SIGINIT / SIGTERM / SIGHUP received
//				running = false
//			case <-p.stopChan:   // Close() called
//				running = false
//			case <-ticker.ch():  // check for a status update
//				buf = p.sync(buf)
//			case <-p.resizeChan: // SIGWINCH received
//				buf = p.handleResize(buf)
//			}
//		}

//		select { // post-loop drain: ensure any final pending tick is flushed before deferred cleanup routines execute
//		case <-ticker.ch():
//			buf = p.sync(buf)
//		default:
//		}
	}

	p.finish(ctx, buf) // render the final frame to the terminal and perform any necessary cleanup
}

var (
	stoppedPrefix = []byte("stopped (")
	oneHundredPct = []byte("100")
)

// finish renders the final progress frame to the terminal.
func (p *Progress) finish(ctx context.Context, buf []byte) {
	buf = buf[:0]
	if err := ctx.Err(); err != nil { // context was aborted via signal, timeout, or parent cancelation
		errStr := err.Error()
		if cause := context.Cause(ctx); cause != nil { errStr = cause.Error() }
		buf = append(buf, p.layout.clearSeq...)
		buf = append(buf, stoppedPrefix...)
		buf = append(buf, errStr...)
		buf = append(buf, ')')
		buf = append(buf, p.layout.doneSeq...)
	} else {                          // clean exit via p.Close() while context still active
		buf = append(buf, p.layout.clearSeq...)
		buf = append(buf, p.layout.prefix...)
		buf = append(buf, oneHundredPct...)
		buf = append(buf, p.layout.suffix...)
		buf = append(buf, p.layout.finalStatus...)
		buf = append(buf, p.layout.doneSeq...)
		if buf[len(buf) - 1] != '\n' {
			buf = append(buf, p.layout.lineTerminator...)
		}
	}

	_, _ = p.output.Write(buf)
}

// helpers for synchronous, deterministic tests

func withClock (c clock ) Option { return func(p *Progress) { p.clock = c } }

type ticker interface {
	ch() <-chan time.Time
	Stop()
}

type clock   interface { tick() ticker     } // enables dependency injection to facilitate synchronous, deterministic testing

type realTicker struct { *time.Ticker      }
type fakeTicker struct { c  chan time.Time }

type realClock  struct { dur time.Duration } // throttles UI updates
type fakeClock  struct { c  chan time.Time } // simulates the passage of time in tests

func (r *realTicker) ch() <-chan time.Time { return r.C }
func (f  fakeTicker) ch() <-chan time.Time { return f.c }

func (r *realClock ) tick() ticker { return &realTicker{ Ticker: time.NewTicker(r.dur) }}
func (f  fakeClock ) tick() ticker { return  fakeTicker(f)                              }

func (f  fakeTicker) Stop() {}
