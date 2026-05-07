// Package progress provides status updates to the terminal as units of work are incrementally completed.
package progress

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

const (
	// scale represents 100% as a large fixed-point integer to support high-precision fractional updates.
	// (the sync/atomic package provides no floating-point types)
	//
	// the choice of 1e15 balances high-precision fractional shares in the context
	// of, e.g., deep recursion, with sufficient uint64 headroom to prevent overflow
	// when performing intermediate percentage calculations (newCurrent * 10000)
	//
	// precision starts to degrade as the total number of work units approaches scale,
	// but even at this limit, each unit of work represents at least 1 unit of scale
	scale    uint64 = 1e15
	minWidth uint16 = 80             // fallback for pipes, redirects, and non-tty outputs
	pctFieldLen     = 3              // the fixed length of the percentage displayed (e.g., "0.0", " 37", "100")
	prefix          = "processing (" // prepended to each progress status line rendered to the terminal
	defaultSuffix   = "%): "         // appended to each percentage status calculation rendered to the terminal
)

// Option defines a functional configuration for Progress.
type Option func(*Progress) // (exported to allow callers to create []*progress.Option to pass to New)

// WithTracker allows callers to explicitly specify a status tracker, e.g.:
//
//   progress.New(ctx, 100, os.Stderr, progress.WithTracker(progress.Fraction))
func WithTracker(s strategy) Option {
	return func(p *Progress) {
		p.tracker = getTracker(s, p.total.Load())
	}
}

// Progress provides a throttled, concurrency-safe, high-precision status indicator for workloads.
type Progress struct {
	// shared state (atomic)
	tracker       statusTracker          // tracks the current progress status
	total         atomic.Uint64          // total work units; 0 for fractional path allocation; > 0 for weight-based accumulation
	current       atomic.Uint64          // accumulates shares of scale as work is completed
	state         atomic.Uint32          // bit-packed word: upper 16 bits for terminal width; lower 16 bits for progress percentage significant digits
	lastState     atomic.Uint32          // previous snapshot of state: used to detect terminal width or progress changes, and skip redundant redraws
	lastStatusVal atomic.Value           // stores the result of the last tracker.load()
	buf           atomic.Pointer[[]byte] // pre-allocated, reusable buffer for writing status messages to the terminal

	// configuration (read-only after construction)
	output        io.Writer      // destination writer for the terminal-formatted work progress status updates
	clock         clock          // provides the timing source for throttled UI updates, allowing for fake clocks in tests
	stopChan      chan struct{}  // signals the background rendering loop to perform final cleanup
	doneChan      chan struct{}  // doneChan is closed once the rendering loop has finished its final draw and cursor restoration
	drawNotify    chan struct{}  // used in tests to signal the completion of a draw cycle
	resizeChan    chan os.Signal // handles terminal window resizing events via the syscall.SIGWINCH signal
	staticWidth   int            // the static width reserved for the prefix prepended to each status message, e.g., "processing (7.4%): "
	clearSeq      string         // ANSI escape sequence used to clear the current terminal line
	doneSeq       string         // ANSI escape sequence used to restore the terminal cursor
	lineTerm      string         // output line terminator
	closeOnce     sync.Once      // closeOnce ensures that cursor restoration and cleanup logic are executed only once
	suffix        string         // appended to each percentage status calculation rendered to the terminal
}

// New initializes a throttled, concurrency-safe, high-precision work progress
// tracker and starts a work completion status rendering loop in the background.
//
// The value of the `totalUnits` parameter determines the accumulation mode used internally:
//
//    pass totalUnits >  0 for weight-based accumulation  (when totalUnits is known)
//    pass totalUnits == 0 for fractional path allocation (when totalUnits is unknown)
func New(ctx context.Context, totalUnits uint64, output io.Writer, opts ...Option) *Progress {
	p := &Progress{
		tracker:     &standardTracker{},
		output:      output,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
		resizeChan:  make(chan os.Signal, 1),
		clock:       &realClock{ dur: 16 * time.Millisecond },
		clearSeq:    "",
		doneSeq:     "\n",
		lineTerm:    "\n",
		suffix:      defaultSuffix,
		staticWidth: len(prefix) + pctFieldLen + len(defaultSuffix), // 12 + 3 + 4 == 19
	}

	p.total.Store(min(totalUnits, scale)) // fall back to scale if totalUnits exceeds max precision

	termWidth := max(getTermWidth(), minWidth)
	buf       := make([]byte, 0, 4 * int(termWidth) + p.staticWidth) // assume worst case where all UTF-8 characters in status strings are 4-bytes each

	p.buf.Store(&buf)
	p.prepareTerminal()
	p.state.Store(uint32(termWidth) << 16)

	for _, opt := range opts { opt(p) } // allows callers to override the default status tracker via the WithTracker Option

	if _, ok := p.tracker.(*percentTracker); ok {
		p.suffix      = "%)"
		p.staticWidth = len(prefix) + pctFieldLen + len(p.suffix) // 12 + 3 + 2 == 17
	}

	signal.Notify(p.resizeChan, syscall.SIGWINCH) // listen for a SIGWINCH signal to handle the terminal window being resized

	go p.renderLoop(ctx)

	return p
}

// InitialBudget returns the full internal scale (100%) to be used as the starting budget for tracking fractional progress.
func (p *Progress) InitialBudget() float64 { return float64(scale) }

// AddTotal dynamically increases the total work budget as new tasks are discovered.
// It is concurrency-safe and ensures the total budget never exceeds scale.
func (p *Progress) AddTotal(n uint64) {
	for {
		oldTotal := p.total.Load()
		newTotal := min(oldTotal + n, scale) // fall back to scale if total exceeds max precision
		if p.total.CompareAndSwap(oldTotal, newTotal) { break }
	}
}

// Report updates the current progress status.
//
//   if total >  0: weight represents the relative weight of the work completed, and the progress percentage is calculated as accumulated weight / totalUnits
//   if total == 0: weight represents the portion of the InitialBudget(), which must be divided among all sub-tasks by the caller
func (p *Progress) Report(weight float64, status string) {
	p.tracker.store(uint64(weight), status)

syncCurrent:
	total := p.total.Load()

	var share uint64
	if total > 0 {
		share = uint64((weight * float64(scale)) / float64(total)) //  weight-based accumulation mode: calculate the share of the total, maintaining precision
	} else {
		share = uint64(weight)                                     // fractional path allocation mode: add the share of the budget directly
	}

	oldCurrent := p.current.Load()
	newCurrent := min(oldCurrent + share, scale) // cap at scale (100%)

	if !p.current.CompareAndSwap(oldCurrent, newCurrent) {
		goto syncCurrent // handle a concurrent Report or AddTotal call
	}

syncState: // derive the UI state from the successfully-committed p.current update above
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

	oldState        := p.state.Load()
	scaledSigDigits := (newCurrent * 10000 + (scale / 2)) / scale
	oldSigDigits    := uint16(oldState        & 0xFFFF)
	newSigDigits    := uint16(scaledSigDigits & 0xFFFF) // satisfy gosec
	newState        := (oldState & 0xFFFF0000) | uint32(max(newSigDigits, oldSigDigits)) // ensure motonicity and preserve termWidth

	if !p.state.CompareAndSwap(oldState, newState) {
		goto syncState // handle concurrent Report call or terminal resize event
	}
}

// Close stops the background renderer and waits for cleanup to complete.
func (p *Progress) Close() {
	p.closeOnce.Do(func() {
		close(p.stopChan) // stop the renderLoop goroutine
		<-p.doneChan      // block until renderLoop exits
	})
}

// renderLoop periodically draws the progress line at ~60 FPS without impeding workers.
func (p *Progress) renderLoop(parentCtx context.Context) {
	ctx, stop := signal.NotifyContext(parentCtx,
		os.Interrupt,    // interrupt signal (ctrl+c)
		syscall.SIGTERM, // kill signal
		syscall.SIGHUP)  // terminal closed signal
	defer stop()         // restore default signal behavior when the loop exits

	ticker := p.clock.tick()
	defer ticker.Stop()

	defer close(p.doneChan)
	defer signal.Stop(p.resizeChan)
	defer p.finish(ctx) // render the final frame to the terminal and perform any necessary cleanup

	if isTerminal(p.output) { _, _ = io.WriteString(p.output, "\033[?25l") } // hide the cursor

	for {
		select {
		case <-ctx.Done():   // parent context canceled, or SIGINIT / SIGTERM / SIGHUP received
			return
		case <-p.stopChan:   // Close() called
			return
		case <-ticker.ch():  // check for a status update
			p.sync()
		case <-p.resizeChan: // SIGWINCH received
			p.handleResize()
		}
	}
}

// sync performs a state-aware redraw, skipping redundant I/O if the progress and status values haven't changed since the last render.
func (p *Progress) sync() { // skips redundant redraws
	currentState := p.state.Load()
	currentVal   := p.tracker.load()
	lastState    := p.lastState.Load()
	lastVal      := p.lastStatusVal.Load()

	if currentState == lastState &&
	   currentVal   == lastVal { return } // status unchanged; skip redundant redraw

	p.draw(currentState, currentVal)

	p.lastState.Store(currentState)
	if currentVal != nil { p.lastStatusVal.Store(currentVal) }
}

// draw formats and renders the current progress status to the terminal, truncating text as needed to fit within the terminal width.
func (p *Progress) draw(state uint32, val any) {
	termWidth := uint16(state >> 16)
	maxLen    := max(int(termWidth) - p.staticWidth, 0)
	status    := p.tracker.value(val)

	truncated := false
	if maxLen == 0 {
		status = ""
	} else {
		status, truncated = truncateFromLeft(status, maxLen) // truncate from left to show most relevant portion (e.g., file basename)
	}

	err := p.writeStatus(uint16(state & 0xFFFF), status, truncated)

	if err == nil && p.drawNotify != nil {
		select {
		case p.drawNotify <- struct{}{}: // ensures synchronous, deterministic tests by signaling the completion of a draw cycle
		default:
		}
	}
}

// writeStatus writes the progress status to to p.output (nominally os.Stderr) using the shared internal buffer to ensure an atomic system call.
func (p *Progress) writeStatus(pctSigDigits uint16, status string, truncated bool) error {
	bufPtr := p.buf.Load()
	if bufPtr == nil { return nil }

	buf := *bufPtr
	buf  = buf[:0]
	buf  = append(buf, p.clearSeq...)
	buf  = append(buf, prefix...)

	switch {
	case pctSigDigits >= 9950: // 99.5% < pctSigDigits >  100% => "100%"
		buf = append(buf, "100"...)
	case pctSigDigits >=  995: // 9.95% < pctSigDigits > 99.4% => " 10%" - " 99%"
		val := (pctSigDigits + 50) / 100 // round to the nearest 1% (995 -> 10; 9949 -> 99)
		buf = append(buf, ' ', byte('0' + (val / 10)),      byte('0' + (val % 10)))
	default:                   // 0.00% < pctSigDigits > 9.94% => "0.1%" - "9.9%"
		val := (pctSigDigits + 5) / 10 // round to the nearest 0.1% (994 -> 9.9; 0 -> 0.0)
		buf = append(buf,      byte('0' + (val / 10)), '.', byte('0' + (val % 10)))
	}

	buf = append(buf, p.suffix...)
	if truncated { buf = append(buf, "..."...) }
	buf = append(buf, status...)
	buf = append(buf, p.lineTerm...)

	_, err := p.output.Write(buf) // single, atomic system call when p.buf <= PIPE_BUF, which p.buf can be reasonably expected to rarely exceed

	return err
}

// prepareTerminal sets the line terminator character and ANSI escape sequences to be used when p.output (nominally os.Stderr) has not been piped or redirected.
func (p *Progress) prepareTerminal() {
	if isTerminal(p.output) {
		p.clearSeq = "\r\033[2K\r" // \033[2K clears the line, \r moves the cursor to the beginning of the line
		p.doneSeq  = "\r\033[?25h" // restores the cursor
		p.lineTerm = ""
	}
}

// finish renders the final progress frame to the terminal.
func (p *Progress) finish(ctx context.Context) {
	cause := context.Cause(ctx)

	var output string
	if cause != nil && !errors.Is(cause, context.Canceled) {
		output = p.clearSeq + "stopped (" + cause.Error() + ")" + p.doneSeq
	} else {
		output = p.clearSeq + "processing (100%): done" + p.doneSeq
	}

	_, _ = io.WriteString(p.output, output)
}

// handleResize records the new terminal width to be respected by subsequent render cycles.
func (p *Progress) handleResize() {
	f, ok := p.output.(*os.File)
	if !ok { return }

	bufPtr   := p.buf.Load()
	newWidth := getResizedTermWidth(f, p.staticWidth)
	reqCap   := 4 * int(newWidth) + p.staticWidth

	if bufPtr == nil || cap(*bufPtr) < reqCap { // grow the buffer when the terminal width is increased
		newBuf := make([]byte, 0, reqCap)
		p.buf.Store(&newBuf)
	}

	for { // atomically update termWidth while preserving concurrent percentage or status changes
		oldState := p.state.Load()
		newState := (oldState & 0xFFFF) | (uint32(newWidth) << 16) // pack newWidth into upper 16 bits, retaining pctSigDigits in lower 16 bits
		if p.state.CompareAndSwap(oldState, newState) { break }
	}
}

// truncateFromLeft constrains the length of progress status messages rendered to the terminal, properly handling utf-8 strings.
func truncateFromLeft(s string, maxLen int) (string, bool) {
	runeCount := utf8.RuneCountInString(s)
	if runeCount <= maxLen { return s, false }

	skip := runeCount - maxLen
	if maxLen > 3 { skip = runeCount - (maxLen - 3) }

	i := 0
	for range skip {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}

	return s[i:], maxLen > 3
}

// getTermWidth determines the width of the terminal window, which is used to format status messages.
func getTermWidth(files ...*os.File) uint16 {
	if len(files) == 0 { files = []*os.File{ os.Stderr } }
	width := int(minWidth)
	for _, f := range files {
		if w, _, err := term.GetSize(int(getFD(f))); err == nil {
			width = max(width, w)
		}
	}
	return uint16(width)
}

// getResizedTermWidth returns the current terminal width, enforcing p.staticWidth as the minimum layout threshold.
func getResizedTermWidth(f *os.File, staticWidth int) uint16 {
	width := staticWidth
	if w, _, err := term.GetSize(int(getFD(f))); err == nil && w > 0 {
		width = max(width, w) // assume a human manually resized the terminal, so support terminal widths as narrow as p.staticWidth
	}
	return uint16(width & 0xFFFF)
}

// isTerminal determines if the specified writer is connected to a terminal.
func isTerminal(v any) bool {
	if testing.Testing() { return false }
	if os.Getenv("GITHUB_ACTIONS") == "true" || // https://docs.github.com/actions/reference/workflows-and-actions/variables
	   os.Getenv("CI"            ) == "true" { return false }
	fd := getFD(v)
	if fd < 0 { return false }
	return term.IsTerminal(fd)
}

// getFD returns the file descriptor of the provided argument.
func getFD(w any) int {
	if f, ok := w.(interface{ Fd() uintptr }); ok { return int(f.Fd()) }
	return -1
}

// helpers for synchronous, deterministic tests

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
func (f *fakeTicker) ch() <-chan time.Time { return f.c }

func (r *realClock ) tick() ticker { return &realTicker{ Ticker: time.NewTicker(r.dur) }}
func (f *fakeClock ) tick() ticker { return &fakeTicker{ c:      f.c                   }}

func (f *fakeTicker) Stop() {}
