// Package progress provides status updates to the terminal as units of work are incrementally completed.
package progress

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

const (
	// scale represents 100% as a large fixed-point integer to support high-precision fractional updates.
	// (the sync/atomic package provides no floating-point types)
	//
	// the choice of 1e12 balances high-precision fractional shares in the context
	// of, e.g., deep recursion, with sufficient uint64 headroom to prevent overflow
	// when performing intermediate percentage calculations (currentVal * 100)
	scale        uint64 = 1e12

	// maxSafeUnits is the maximum allowable number of work units before intermediate percentage
	// calculations risk uint64 overflow; some precision will be lost when totalUnits > maxSafeUnits
	// TODO(jeff): remove the artificial cap in favor of a superior approach to
	//             internal calculations which remain safe from underflow & overflow
	maxSafeUnits uint64 = math.MaxUint64 / scale // ~18.4M

	minWidth     uint16 = 80     // fallback for pipes, redirects, and non-tty outputs

	pctFieldLen = 3              // the fixed length of the percentage displayed (e.g., "0.0", " 37", "100")
	prefix      = "processing (" // prepended to each progress status line rendered to the terminal
	suffix      = "%): "         // appended to each percentage status calculation rendered to the terminal
)

// Progress provides a throttled, concurrency-safe, high-precision status indicator for workloads.
type Progress struct {
	// shared state (atomic)
	total          atomic.Uint64          // 0 for fractional path allocation; > 0 for weight-based accumulation
	current        atomic.Uint64          // accumulates shares of scale
	state          atomic.Uint32          // bit-packed word: upper 16 bits for terminal width; lower 16 bits for progress percentage significant digits
	lastState      atomic.Uint32          // previous snapshot of state: used to detect terminal width or progress changes, and skip redundant redraws
	statusText     atomic.Pointer[string] // pointer to the current status message rendered to the terminal
	lastStatusText atomic.Pointer[string] // pointer to the previous status message, used to determine if the text content changed since the last render cycle

	// configuration (read-only after construction)
	buf            []byte                 // reusable buffer for writing status messages to the terminal
	output         io.Writer              // destination writer for the terminal-formatted work progress status updates
	clock          clock                  // provides the timing source for throttled UI updates, allowing for fake clocks in tests
	stopChan       chan struct{}          // signals the background rendering loop to perform final cleanup
	doneChan       chan struct{}          // doneChan is closed once the rendering loop has finished its final draw and cursor restoration
	drawNotify     chan struct{}          // used in tests to signal the completion of a draw cycle
	resizeChan     chan os.Signal         // handles terminal window resizing
	staticWidth    int                    // the static width reserved for the prefix prepended to each status message, e.g., "processing (7.4%): "
	clearSeq       string                 // ANSI escape sequence used to clear the current terminal line
	doneSeq        string                 // ANSI escape sequence used to restore the terminal cursor
	lineTerm       string                 // output line terminator
	closeOnce      sync.Once              // closeOnce ensures that cursor restoration and cleanup logic are executed only once
}

// New initializes a throttled, concurrency-safe, high-precision work progress
// tracker and starts a work completion status rendering loop in the background.
//
// The value of the `totalUnits` parameter determines the accumulation mode used internally:
//
//    pass totalUnits >  0 for weight-based accumulation  (when totalUnits is known a priori)
//    pass totalUnits == 0 for fractional path allocation (when totalUnits is not known a priori)
func New(ctx context.Context, totalUnits uint64, output io.Writer) *Progress {
	p := &Progress{
		output:      output,
		buf:         make([]byte, 0, 128), // pre-allocate to avoid heap growth during draw() cycles
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
		resizeChan:  make(chan os.Signal, 1),
		clock:       &realClock{ dur: 16 * time.Millisecond },
		clearSeq:    "",
		doneSeq:     "\n",
		lineTerm:    "\n",
		staticWidth: len(prefix) + pctFieldLen + len(suffix), // 12 + 3 + 4 == 19
	}

	termWidth, useANSI := p.detectTerminal()

	p.total.Store(min(totalUnits, maxSafeUnits)) // fall back to maxSafeUnits if totalUnits exceeds max precision
	p.state.Store(uint32(termWidth) << 16)
	p.statusText.Store(new(""))

	signal.Notify(p.resizeChan, syscall.SIGWINCH) // trap SIGWINCH to handle the terminal window being resized

	go p.renderLoop(ctx, useANSI)

	return p
}

// InitialBudget returns the full internal scale (100%) to be used as the starting budget for tracking fractional progress.
func (p *Progress) InitialBudget() float64 { return float64(scale) }

// AddTotal dynamically increases the total work budget as new tasks are discovered.
// It is concurrency-safe and can be called concurrently with Report().
func (p *Progress) AddTotal(n uint64) {
	p.total.Add(min(n, maxSafeUnits)) // fall back to maxSafeUnits if totalUnits exceeds max precision
}

// Report updates the current progress and status.
//
//   if total >  0: n represents the relative weight of the work completed, and the progress percentage is calculated as n / totalUnits
//   if total == 0: n represents the portion of the InitialBudget(), which must be divided among all sub-tasks by the caller
func (p *Progress) Report(n float64, status string) {
	total := p.total.Load()

	var share uint64
	if total > 0 {
		share = uint64((n / float64(total)) * float64(scale)) //  weight-based accumulation mode: calculate the share of the total
	} else {
		share = uint64(n)                                     // fractional path allocation mode: add the budget share directly
	}
	newCurrent := p.current.Add(share)
	if newCurrent > scale { // cap at scale (100%)
		newCurrent = scale
		p.current.Store(scale)
	}

	// update numeric state values while avoiding new memory allocations to mitigate GC pressure
	//
	// ((newCurrent * 10000 + (scale / 2)) / scale) converts the 1e12 scale to 4 significant digits (1e4).
	// adding half of the total scale (the divisor) ensures precise rounding to avoid floor truncation during integer division.
	newSigDigits := uint16((newCurrent * 10000 + (scale / 2)) / scale)
	oldState     := p.state.Load()
	currentWidth := uint16(oldState >> 16) // preserve termWidth while updating state
	p.state.Store(uint32(currentWidth) << 16 | uint32(newSigDigits))

	// update the status string, allocating only if its content changes
	if oldStatusText := p.statusText.Load(); oldStatusText == nil || *oldStatusText != status {
		p.statusText.Store(&status)
	}
}

// Close stops the background renderer, writes the final completion frame, and restores the terminal cursor if needed.
func (p *Progress) Close(ctx context.Context) {
	p.closeOnce.Do(func() {
		close(p.stopChan) // stop the background renderLoop
		<-p.doneChan      // block until renderLoop exits
		p.finish(ctx)
	})
}

// renderLoop periodically draws the progress line at ~60 FPS without impeding the processing logic.
func (p *Progress) renderLoop(ctx context.Context, useANSI bool) {
	if useANSI { _, _ = io.WriteString(p.output, "\033[?25l") } // hide the cursor

	ticker := p.clock.tick()
	defer ticker.Stop()

	defer close(p.doneChan)
	defer signal.Stop(p.resizeChan)

	for {
		select {
		case <-ctx.Done():   // parent context canceled
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

// sync compares the current progress and status text against the last rendered values to skip redundant redraws
func (p *Progress) sync() { // skips redundant redraws
	lastState         := p.lastState.Load()
	lastStatusText    := p.lastStatusText.Load()
	currentState      := p.state.Load()
	currentStatusText := p.statusText.Load()

	if currentState      == lastState &&
	   currentStatusText == lastStatusText { return }

	p.draw(currentState, currentStatusText)
	p.lastState.Store(currentState)
	p.lastStatusText.Store(currentStatusText)
}

// draw formats and renders the current progress status to the terminal, truncating text as needed to fit within the terminal width.
func (p *Progress) draw(state uint32, statusTextPtr *string) {
	termWidth    := uint16(state >> 16)
	maxLen       := max(int(termWidth) - p.staticWidth, 0)
	status       := ""
	if statusTextPtr != nil { status = *statusTextPtr }

	truncated := false
	if maxLen == 0 {
		status = ""
	} else {
		status, truncated = truncateFromLeft(status, maxLen) // truncate from left to show most relevant portion (e.g., file basename)
	}

	err := p.writeStatus(uint16(state & 0xFFFF), status, truncated)

	if err == nil && p.drawNotify != nil {
		select {
		case p.drawNotify <- struct{}{}: // ensures deterministic tests by signaling the completion of a draw cycle
		default:
		}
	}
}

// writeStatus writes the progress status to to p.output (nominally the terminal's stderr) using the shared internal buffer to ensure an atomic system call.
func (p *Progress) writeStatus(pctSigDigits uint16, status string, truncated bool) error {
	p.buf = p.buf[:0]
	p.buf = append(p.buf, p.clearSeq...)
	p.buf = append(p.buf, prefix...)

	switch {
	case pctSigDigits >= 9995:
		p.buf = append(p.buf, "100"...)
	case pctSigDigits >=  995:
		val  := pctSigDigits / 100
		p.buf = append(p.buf, ' ', byte('0' + (val          / 10  )),      byte('0' + (val                 % 10))) // " xy"
	default:
		p.buf = append(p.buf,      byte('0' + (pctSigDigits / 1000)), '.', byte('0' + (pctSigDigits / 100) % 10))  // "x.y"
	}

	p.buf = append(p.buf, suffix...)
	if truncated { p.buf = append(p.buf, "..."...) }
	p.buf = append(p.buf, status...)
	p.buf = append(p.buf, p.lineTerm...)

	_, err := p.output.Write(p.buf) // single, atomic system call when p.buf <= 4kB, which p.buf can be reasonably expected to never exceed

	return err
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

// detectTerminal determines if p.output has been piped or redirected to transparently handle those cases.
func (p *Progress) detectTerminal() (uint16, bool) {
	f, ok := p.output.(*os.File)
	if !ok { return minWidth, false }

	termWidth := getTermWidth(f)
	useANSI   := false

	if fd := f.Fd(); fd <= math.MaxInt && term.IsTerminal(int(fd)) { // fd <= math.MaxInt satisfies gosec
		useANSI    = true
		p.clearSeq = "\r\033[2K\r" // \033[2K clears the line, \r moves the cursor to the beginning of the line
		p.doneSeq  = "\r\033[?25h" // restores the cursor
		p.lineTerm = ""
	}

	return termWidth, useANSI
}

// handleResize records the new terminal width to be respected by subsequent render cycles.
func (p *Progress) handleResize() {
	f, ok := p.output.(*os.File)
	if !ok { return }

	newWidth := getTermWidth(f)

	for { // atomically update termWidth while preserving concurrent percentage or status changes
		oldState     := p.state.Load()
		curSigDigits :=  uint16(oldState & 0xFFFF)                      // drop upper 16 bits (old termWidth), and preserve lower 16 bits (pctSigDigits)
		newState     := (uint32(newWidth) << 16) | uint32(curSigDigits) // pack newWidth into upper 16 bits, retaining pctSigDigits in lower 16 bits
		if p.state.CompareAndSwap(oldState, newState) { break }
	}
}

// finish renders the final progress frame to the terminal.
func (p *Progress) finish(ctx context.Context) {
	output := p.doneSeq // newline or cursor resotring ANSI escape sequence
	cause  := context.Cause(ctx)

	if cause != nil && !errors.Is(cause, context.Canceled) {
		output = p.clearSeq + "stopped (" + cause.Error() + ")" + p.doneSeq
	} else {
		state      := p.state.Load()
		statusText := p.statusText.Load()
		pct        := uint16(state & 0xFFFF) // unpack pctSigDigits from the lower 16 bits
		if statusText == nil    ||
		  *statusText != "done" ||
		   pct        <  10000  {
			output = p.clearSeq + "processing (100%): done" + p.doneSeq
		}
	}
	_, _ = io.WriteString(p.output, output)
}

type ticker interface {
	ch() <-chan time.Time
	Stop()
}

type clock   interface { tick() ticker     } // enables dependency injection to facilitate testing

type realTicker struct { *time.Ticker      }
type fakeTicker struct { c  chan time.Time }

type realClock  struct { dur time.Duration } // throttles UI updates
type fakeClock  struct { c  chan time.Time } // simulates the passage of time in tests

func (r *realTicker) ch() <-chan time.Time { return r.C }
func (f *fakeTicker) ch() <-chan time.Time { return f.c }

func (r *realClock ) tick() ticker { return &realTicker{ Ticker: time.NewTicker(r.dur) }}
func (f *fakeClock ) tick() ticker { return &fakeTicker{ c:      f.c                   }}

func (f *fakeTicker) Stop() {}

// getTermWidth determines the width of the terminal window, which is used to format status messages.
func getTermWidth(files ...*os.File) uint16 {
	if len(files) == 0 {
		files = []*os.File{
			os.Stdout, // Fd() == 1
			os.Stderr, // Fd() == 2
			os.Stdin,  // Fd() == 0
		}
	}
	width := int(minWidth)
	for _, f := range files {
		// although f.Fd() is 0 (os.Stdin), 1 (os.Stdout), or 2 (os.Stderr), the
		// following check is performed to satisfy the gosec linter (otherwise
		// gosec complains about possible integer overflow in the call to int())
		if fd := f.Fd(); fd <= math.MaxInt {
			if w, _, err := term.GetSize(int(fd)); err == nil {
				width = max(width, w)
			}
		}
	}
	return uint16(width)
}
