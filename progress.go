// Package progress provides status updates to the terminal as units of work are incrementally completed.
package progress

import (
	"io"
	"math"
	"os"
	"os/signal"
	"strconv"
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
	// maxSafeUnits is the maximum number of work units allowed before intermediate percentage
	// calculations risk uint64 overflow; some precision will be lost when totalUnits > maxSafeUnits
	maxSafeUnits uint64 = math.MaxUint64 / scale
)

// Progress provides a throttled, concurrency-safe, high-precision status indicator for workloads.
type Progress struct {
	total      atomic.Uint64  // 0 for fractional path allocation; > 0 for weight-based accumulation
	mu         sync.Mutex     // synchronizes terminal I/O and UI state updates
	buf        []byte         // reusable buffer for writing status messages to the terminal
	output     io.Writer      // destination writer for the terminal-formatted work progress status updates
	input      atomic.Value   // stores the latest unit of work being processed
	current    atomic.Uint64  // accumulates shares of scale
	lastWidth  int            // the last terminal width used for drawing (used to skip redundant UI updates)
	lastPct    string         // the last rendered percentage string      (used to skip redundant UI updates)
	lastStatus string         // the last rendered status message         (used to skip redundant UI updates)
	stopChan   chan struct{}  // signals the background rendering loop to perform final cleanup and exit
	doneChan   chan struct{}  // doneChan is closed once the rendering loop has finished its final draw and cursor restoration
	drawNotify chan struct{}  // drawNotify is used in tests to signal the completion of a draw cycle
	resizeChan chan os.Signal // handles terminal window resizing
	clock      clock          // provides the timing source for throttled UI updates, allowing for fake clocks in tests
	width      int            // the width of the terminal window; updated by a syscall.SIGWINCH listener
	closeOnce  sync.Once      // closeOnce ensures that cursor restoration and cleanup logic are executed only once
	clearSeq   string         // ANSI escape sequence used to clear the current terminal line
	doneSeq    string         // ANSI escape sequence used to restore the terminal cursor
	lineTerm   string         // output line terminator
	prefixLen  int            // length (number of characters) of the static prefix string ("processing (")
}

type clock interface { tick() <-chan time.Time } // enables dependency injection to facilitate testing

type realClock struct { dur time.Duration  }     // throttles UI updates
func (r *realClock) tick() <-chan time.Time { return time.NewTicker(r.dur).C }

type fakeClock struct { chn chan time.Time }     // simulates the passage of time in tests
func (f *fakeClock) tick() <-chan time.Time { return f.chn }

const (
	procPrefix = "processing ("
	procSuffix = "%): "
)

// NewProgress initializes a throttled, concurrency-safe, high-precision work progress
// tracker and starts a work completion status rendering loop in the background.
//
// The value of the `totalUnits` parameter determines the accumulation mode used internally:
//
//    pass totalUnits >  0 for weight-based accumulation  (when totalUnits is known a priori)
//    pass totalUnits == 0 for fractional path allocation (when totalUnits is not known a priori)
func NewProgress(totalUnits uint64, output io.Writer) *Progress {
	safeTotal := min(totalUnits, maxSafeUnits) // fall back to maxSafeUnits if totalUnits exceeds max precision

	useANSI       := false
	clearSeq      := ""
	doneSeq       := "\n"
	lineTerm      := "\n"
	prefixLen     := 15
	terminalWidth := 80
	if f, ok := output.(*os.File); ok {
		terminalWidth = getWidth(f)
		fd := f.Fd()
		if fd <= math.MaxInt {
			if term.IsTerminal(int(fd)) {
				useANSI   = true
				clearSeq  = "\r\033[2K\r" // \033[2K clears the line, \r moves the cursor to the beginning of the line
				doneSeq   = "\r\033[?25h" // restores the cursor
				prefixLen = 20            // len(ansiClear) + len(procPrefix) + len(procSuffix)
				lineTerm  = ""
			}
		}
	}

	p := &Progress{
		buf:        make([]byte, 0, 128), // pre-allocate to avoid heap growth during draw()
		output:     output,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
		resizeChan: make(chan os.Signal, 1),
		clock:      &realClock{ dur: 16 * time.Millisecond },
		width:      max(terminalWidth, 80),
		clearSeq:   clearSeq,
		doneSeq:    doneSeq,
		lineTerm:   lineTerm,
		prefixLen:  prefixLen,
	}
	p.total.Store(safeTotal)
	p.input.Store("")

	if useANSI {
		_, _ = io.WriteString(p.output, "\033[?25l") // hide the cursor
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM) // trap SIGINT and SIGTERM so the hidden cursor can be restored
	signal.Notify(p.resizeChan, syscall.SIGWINCH)         // trap SIGWINCH to handle the terminal window being resized

	go func() {
		defer signal.Stop(sigChan) // clean up signal listener
		select {
		case <-sigChan:            // SIGINT or SIGTERM trapped...
			p.restoreAndExit()     // ...restore the cursor before exiting
		case <-p.stopChan:
			return                 // normal exit triggered by Close()
		}
	}()

	go p.renderLoop()
	return p
}

// getWidth determines the width of the terminal window, which is used to format status messages.
func getWidth(files ...*os.File) int {
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

// writeStatus writes the progress status to to p.output (nominally the terminal's stderr) using the shared internal buffer to ensure an atomic system call.
// callers must acquire p.mu before calling writeStatus to protect the shared buffer and ensure UI consistency.
func (p *Progress) writeStatus(percent, status string) error {
	p.buf = p.buf[:0]
	p.buf = append(p.buf, p.clearSeq...)
	p.buf = append(p.buf, procPrefix...)
	p.buf = append(p.buf, percent...)
	p.buf = append(p.buf, procSuffix...)
	p.buf = append(p.buf, status...)
	p.buf = append(p.buf, p.lineTerm...)
	_, err := p.output.Write(p.buf) // single, atomic system call
	return err
}

// InitialBudget returns the full internal scale (100%) to be used as the starting budget for tracking fractional progress.
func (p *Progress) InitialBudget() float64 { return float64(scale) }

// AddTotal dynamically increases the total work budget as new tasks are discovered.
// It is concurrency-safe and can be called concurrently with Report().
func (p *Progress) AddTotal(n uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.total.Add(n)
}

// Report updates the current progress and status.
//
//   if total >  0: n represents the relative weight of the work completed, and the progress percentage is calculated as n / totalUnits
//   if total == 0: n represents the portion of the InitialBudget(), which must be divided among all sub-tasks by the caller
func (p *Progress) Report(n float64, status string) {
	p.input.Store(status)

	total := p.total.Load()
	var share uint64
	if total > 0 {
		share = uint64((n / float64(total)) * float64(scale)) // weight-based accumulation mode: calculate the share of the total
	} else {
		share = uint64(n)                                     // fractional path allocation mode: add the budget share directly
	}
	if p.current.Add(share) > scale { // cap at scale (100%)
		p.current.Store(scale)
	}
}

// renderLoop periodically draws the progress line at ~60 FPS without impeding the processing logic.
func (p *Progress) renderLoop() {
	tickerChan := p.clock.tick()
	for {
		select {
		case <-p.resizeChan:
			if f, ok := p.output.(*os.File); ok {
				p.width = getWidth(f)
			}
		case <-tickerChan:
			p.draw()
		case <-p.stopChan:
			close(p.doneChan)
			return
		}
	}
}

// draw clears the current terminal line and prints the formatted percentage and status string, truncating text as needed to fit within the terminal width.
func (p *Progress) draw() {
	p.mu.Lock()
	currentVal := p.current.Load()
	status, _  := p.input.Load().(string)
	width      := p.width
	lastWidth  := p.lastWidth
	lastPct    := p.lastPct
	lastStatus := p.lastStatus
	p.mu.Unlock()

	defer func() {
		if p.drawNotify != nil { // enables fast and deterministic tests
			p.drawNotify <- struct{}{}
		}
	}()

	percent := (float64(currentVal) * 100.0) / float64(scale) // multiply before dividing for precision; safe from uint64 overflow when currentVal <= ~1.8e17

	if percent >= 100 { return } // Close() renders the final completion frame

	var pctBuf [8]byte // temporary small stack buffer for the float
	var pctStr string  // formatted percentage (unfortunately %3g%% doesn't quite work)
	switch {
	case percent >= 99.95:
		pctStr = "100"
	case percent >=  9.95:
		n := len(strconv.AppendFloat(pctBuf[1:1], percent, 'f', 0, 64))
		if n < 3 {
			pctBuf[0] = ' '
			pctStr    = string(pctBuf[ :1 + n])
		} else {
			pctStr    = string(pctBuf[1:1 + n])
		}
	default:
		pctStr = string(strconv.AppendFloat(pctBuf[:0], percent, 'f', 1, 64))
	}

	maxLen := max(width - (p.prefixLen + len(pctStr)), 0)

	switch {
	case maxLen == 0:
		status = ""
	case len(status) > maxLen && maxLen > 3:
		status = "..." + status[len(status) - maxLen + 3:] // truncate from left to show most relevant portion (e.g., file basename)
	case len(status) > maxLen:
		status = status[:maxLen]
	}

	// TODO(jeff): move this check earlier in the body of this method to minimize unnecessary
	//             work done in the case of a redundant UI update to make the hot path faster
	//
	//             perhaps by using functional analogues of pctStr (i.e., percent, but to the
	//             number of significant digits printed to the terminal, not the float64 value)
	//             and status (i.e., without the truncation)
	if width  == lastWidth &&
	   pctStr == lastPct   &&
	   status == lastStatus {
		return // skip redundant UI updates
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	err := p.writeStatus(pctStr, status)

	if err == nil {
		p.lastWidth  = width
		p.lastPct    = pctStr
		p.lastStatus = status
	}
}

// restoreAndExit restores the cursor upon trapping a SIGINT or SIGTERM signal.
func (p *Progress) restoreAndExit() {
	p.Close()
	os.Exit(1)
}

// Close stops the background renderer, writes the final completion frame, and restores the terminal cursor if needed.
func (p *Progress) Close() {
	p.closeOnce.Do(func() {
		close(p.stopChan) // stop the background renderLoop
		<-p.doneChan      // block until renderLoop exits
		p.mu.Lock()
		defer p.mu.Unlock()
		_, _ = io.WriteString(p.output, p.clearSeq + "processing (100%): done" + p.doneSeq)
	})
}
