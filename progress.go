// Package progress provides status updates to the terminal as units of work are incrementally completed.
package progress

import (
	"io"
	"math"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/term"
)

// This package largely represents a personal experiment born of curiosity
// more than a practical solution to a UX issue for its parent module,
// because the main program already processes fairly substantial codebases
// like k8s within < ~1 second on my 2020 first-generation M1 MacBook Air.

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
	maxSafeUnits uint64 = math.MaxUint64 / scale

	pctFieldLen = 3              // the fixed length of the percentage displayed (e.g., "0.0", " 37", "100")
	prefix      = "processing (" // prepended to each progress status line rendered to the terminal
	suffix      = "%): "         // appended to each percentage status calculation rendered to the terminal
)

// Progress provides a throttled, concurrency-safe, high-precision status indicator for workloads.
type Progress struct {
	// shared state (atomic)
	total       atomic.Uint64            // 0 for fractional path allocation; > 0 for weight-based accumulation
	current     atomic.Uint64            // accumulates shares of scale
	state       atomic.Pointer[snapshot] // stores an optimized representation of the progress status      (used to skip redundant UI updates)
	lastState   atomic.Pointer[snapshot] // stores an optimized representation of the last rendered status (used to skip redundant UI updates)

	// configuration (read-only after NewProgress)
	buf         []byte         // reusable buffer for writing status messages to the terminal
	output      io.Writer      // destination writer for the terminal-formatted work progress status updates
	clock       clock          // provides the timing source for throttled UI updates, allowing for fake clocks in tests
	stopChan    chan struct{}  // signals the background rendering loop to perform final cleanup and exit
	doneChan    chan struct{}  // doneChan is closed once the rendering loop has finished its final draw and cursor restoration
	drawNotify  chan struct{}  // drawNotify is used in tests to signal the completion of a draw cycle
	resizeChan  chan os.Signal // handles terminal window resizing
	staticWidth int            // the static width reserved for the prefix prepended to each status message, e.g., "processing (7.4%): "
	closeOnce   sync.Once      // closeOnce ensures that cursor restoration and cleanup logic are executed only once
	clearSeq    string         // ANSI escape sequence used to clear the current terminal line
	doneSeq     string         // ANSI escape sequence used to restore the terminal cursor
	lineTerm    string         // output line terminator
}

// NewProgress initializes a throttled, concurrency-safe, high-precision work progress
// tracker and starts a work completion status rendering loop in the background.
//
// The value of the `totalUnits` parameter determines the accumulation mode used internally:
//
//    pass totalUnits >  0 for weight-based accumulation  (when totalUnits is known a priori)
//    pass totalUnits == 0 for fractional path allocation (when totalUnits is not known a priori)
func NewProgress(totalUnits uint64, output io.Writer) *Progress {
	useANSI   := false
	clearSeq  := ""
	doneSeq   := "\n"
	lineTerm  := "\n"
	termWidth := 80
	if f, ok := output.(*os.File); ok {
		fd       := f.Fd()
		termWidth = getTermWidth(f)
		if fd <= math.MaxInt {
			if term.IsTerminal(int(fd)) {
				useANSI  = true
				clearSeq = "\r\033[2K\r" // \033[2K clears the line, \r moves the cursor to the beginning of the line
				doneSeq  = "\r\033[?25h" // restores the cursor
				lineTerm = ""
			}
		}
	}
	p := &Progress{
		output:      output,
		buf:         make([]byte, 0, 128), // pre-allocate to avoid heap growth during draw() cycles
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
		resizeChan:  make(chan os.Signal, 1),
		clock:       &realClock{ dur: 16 * time.Millisecond },
		clearSeq:    clearSeq,
		doneSeq:     doneSeq,
		lineTerm:    lineTerm,
		staticWidth: len(prefix) + pctFieldLen + len(suffix), // 12 + 3 + 4 == 19
	}

	p.total.Store(min(totalUnits, maxSafeUnits)) // fall back to maxSafeUnits if totalUnits exceeds max precision
	p.state.Store(&snapshot{
		termWidth: uint16(max(termWidth, 80)),
	})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,      syscall.SIGTERM, os.Interrupt) // trap SIGTERM and SIGINT so the hidden cursor can be restored
	signal.Notify(p.resizeChan, syscall.SIGWINCH)              // trap SIGWINCH to handle the terminal window being resized

	go func() {
		defer signal.Stop(sigChan) // clean up signal listener
		select {
		case <-sigChan:            // SIGTERM or SIGINT trapped...
			p.restoreAndExit()     // ...restore the cursor before exiting
		case <-p.stopChan:
			return                 // normal exit triggered by Close()
		}
	}()

	go p.renderLoop(useANSI)
	return p
}

// InitialBudget returns the full internal scale (100%) to be used as the starting budget for tracking fractional progress.
func (p *Progress) InitialBudget() float64 { return float64(scale) }

// AddTotal dynamically increases the total work budget as new tasks are discovered.
// It is concurrency-safe and can be called concurrently with Report().
func (p *Progress) AddTotal(n uint64) { p.total.Add(n) }

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
	// ((newCurrent * 10000 + (scale / 2)) / scale) converts the 1e12 scale to 4 significant digits (1e4).
	// adding half of the total scale (the divisor) ensures precise rounding to avoid floor truncation during integer division.
	newSigDigits := uint16((newCurrent * 10000 + (scale / 2)) / scale)
	for { // CAS loop to ensure termWidth is not overwritten by a concurrent update
		oldState := p.state.Load()
		newState := &snapshot{
			input:        status,
			pctSigDigits: newSigDigits,
			termWidth:    oldState.termWidth,
		}
		if p.state.CompareAndSwap(oldState, newState) { break }
	}
}

// Close stops the background renderer, writes the final completion frame, and restores the terminal cursor if needed.
func (p *Progress) Close() {
	p.closeOnce.Do(func() {
		close(p.stopChan)      // stop the background renderLoop
		<-p.doneChan           // block until renderLoop exits
		output    := p.doneSeq // newline or cursor resotring ANSI escape sequence
		lastState := p.lastState.Load()
		if lastState              == nil    ||
		   lastState.input        != "done" ||
		   lastState.pctSigDigits <  10000  {
			output = p.clearSeq + "processing (100%): done" + p.doneSeq
		}
		_, _ = io.WriteString(p.output, output)
	})
}

// renderLoop periodically draws the progress line at ~60 FPS without impeding the processing logic.
func (p *Progress) renderLoop(useANSI bool) {
	if useANSI { _, _ = io.WriteString(p.output, "\033[?25l") } // hide the cursor
	tickerChan := p.clock.tick()
	for {
		select {
		case <-p.resizeChan:
			if f, ok := p.output.(*os.File); ok {
				width := getTermWidth(f)
				if width > math.MaxInt { width = 80 }
				newWidth := uint16(width)
				for { // atomically update termWidth while preserving concurrent percentage or status changes
					oldState          := p.state.Load()
					newState          := *oldState // shallow copy existing data
					newState.termWidth = newWidth
					if p.state.CompareAndSwap(oldState, &newState) { break }
				}
			}
		case <-tickerChan:
			lastState    := p.lastState.Load()
			currentState := p.state.Load()
			if currentState == lastState { continue }
			if lastState != nil                                    &&
			   currentState.pctSigDigits == lastState.pctSigDigits &&
			   currentState.input        == lastState.input        &&
			   currentState.termWidth    == lastState.termWidth    {
					 p.lastState.Store(currentState)
					 continue
			}
			p.draw(currentState)
			p.lastState.Store(currentState)
		case <-p.stopChan:
			close(p.doneChan)
			return
		}
	}
}

// draw formats and renders the current progress status to the terminal, truncating text as needed to fit within the terminal width.
func (p *Progress) draw(s *snapshot) {
	pctSigDigits := s.pctSigDigits
	status       := s.input
	maxLen       := max(int(s.termWidth) - p.staticWidth, 0)

	switch {
	case maxLen == 0:
		status = ""
	case len(status) > maxLen && maxLen > 3:
		status = "..." + status[len(status) - maxLen + 3:] // truncate from left to show most relevant portion (e.g., file basename)
	case len(status) > maxLen:
		status = status[:maxLen]
	}

	err := p.writeStatus(pctSigDigits, status)

	if err == nil && p.drawNotify != nil {
		select {
		case p.drawNotify <- struct{}{}:
		default:
		}
	}
}

// writeStatus writes the progress status to to p.output (nominally the terminal's stderr) using the shared internal buffer to ensure an atomic system call.
func (p *Progress) writeStatus(digits uint16, status string) error {
	p.buf = p.buf[:0]
	p.buf = append(p.buf, p.clearSeq...)
	p.buf = append(p.buf, prefix...)

	switch {
	case digits >= 9995:
		p.buf = append(p.buf, "100"...)
	case digits >=  995:
		val  := digits / 100
		p.buf = append(p.buf, ' ', byte('0' + (val    / 10  )),      byte('0' + (val           % 10))) // " xy"
	default:
		p.buf = append(p.buf,      byte('0' + (digits / 1000)), '.', byte('0' + (digits / 100) % 10))  // "x.y"
	}

	p.buf = append(p.buf, suffix...)
	p.buf = append(p.buf, status...)
	p.buf = append(p.buf, p.lineTerm...)

	_, err := p.output.Write(p.buf) // single, atomic system call

	return err
}

// restoreAndExit performs an orderly shutdown upon receiving a termination signal (SIGTERM or SIGINT),
// calling p.Close() to synchronize with the renderer, ensuring the final state is
// written to the terminal, and restoring the cursor (if necessary) before exiting.
func (p *Progress) restoreAndExit() {
	p.Close()
	os.Exit(1)
}

// snapshot represents an immutable, point-in-time state of the progress tracker.
// It is designed for lock-free pointer swapping via atomic.Pointer.
// On 64-bit systems, this fits within 24 bytes (16 for string, 4 for the two
// integers, plus 4 bytes of unused memory), ensuring it resides within a single
// CPU cache line to minimize memory latency during high-frequency comparisons.
type snapshot struct {
	input        string // the latest status message
	pctSigDigits uint16 // the first 4 significant digits (e.g., 1234 for 12.34%)
	termWidth    uint16 // the width of the terminal at the time of the snapshot
}

type clock interface { tick() <-chan time.Time } // enables dependency injection to facilitate testing

type realClock struct { dur time.Duration  } // throttles UI updates
func (r *realClock) tick() <-chan time.Time { return time.NewTicker(r.dur).C }

type fakeClock struct { chn chan time.Time } // simulates the passage of time in tests
func (f *fakeClock) tick() <-chan time.Time { return f.chn }

// percentSigDigits returns the first four significant digits of the given value.
// func percentSigDigits(val uint64) uint16 {
//	if val == 0 { return 0 }
//	v := val
//	if v >= 1000 {
//		for v >= 10000 {
//			v /= 10
//		}
//	} else {
//		for v < 1000 && v > 0 {
//			v *= 10
//		}
//	}
//	return uint16(v)
// }

// getTermWidth determines the width of the terminal window, which is used to format status messages.
func getTermWidth(files ...*os.File) int {
	width := 80
	if len(files) == 0 {
		files = []*os.File{
			os.Stdout, // Fd() == 1
			os.Stderr, // Fd() == 2
			os.Stdin,  // Fd() == 0
		}
	}
	for _, f := range files {
		fd := f.Fd()
		// although f.Fd() is 0 (os.Stdin), 1 (os.Stdout), or 2 (os.Stderr), the
		// following check is performed to satisfy the gosec linter (otherwise
		// gosec complains about possible integer overflow in the call to int())
		if fd > math.MaxInt { continue } // skip if FD is logically impossible for term.GetSize (really, just making gosec happy)
		if w, _, err := term.GetSize(int(fd)); err == nil {
			if w > width { width = w }
		}
	}
	return max(width, 80) // fallback for pipes, redirects, and non-tty outputs
}
