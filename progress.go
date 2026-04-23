// Package progress provides status updates for units of work being processed.
package progress

import (
	"fmt"
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

// this package largely represents a personal experiment born of curiosity
// more than a practical solution to a UX issue for its parent module,
// because the main program already processes fairly substantial code bases
// like k8s within < ~2 seconds on my 2020 first-gen M1 MacBook Air.
//
// it may evolve into a generic mini library if i can find a more sophisticated
// method of accurately reporting progress status updates that are actually
// proportional to the total work underway. but as it currently stands, when
// running in weight-based accumulation mode, its assumption that each unit
// of work (e.g. the work of processing the coverage data for a given Go
// source file) costs the same amount of computational resource is very crude
// and naïve, and ultimately results in inaccurate progress status updates.

// TODO(jeff): figure out how to simplify the interface to this package
//             by storing more state to obviate callers being
//             responsible for tracking work completion status

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
	resizeChan chan os.Signal // handles terminal window resizing
	clock      clock          // provides the timing source for throttled UI updates, allowing for fake clocks in tests
	width      int            // the width of the terminal window; updated by a syscall.SIGWINCH listener
	drawNotify chan struct{}  // drawNotify is used in tests to signal the completion of a draw cycle
	closeOnce  sync.Once      // closeOnce ensures that cursor restoration and cleanup logic are executed only once
}

type clock interface { tick() <-chan time.Time } // enables dependency injection to facilitate testing

type realClock struct { dur time.Duration  }     // throttles UI updates
func (r *realClock) tick() <-chan time.Time { return time.NewTicker(r.dur).C }

type fakeClock struct { chn chan time.Time }     // simulates the passage of time in tests
func (f *fakeClock) tick() <-chan time.Time { return f.chn }

// NewProgress initializes a throttled, concurrency-safe, high-precision work progress
// tracker and starts a work completion status rendering loop in the background.
//
// The value of the `totalUnits` parameter determines the accumulation mode used internally:
//
//    pass totalUnits >  0 for weight-based accumulation  (when totalUnits is known a priori)
//    pass totalUnits == 0 for fractional path allocation (when totalUnits is not known a priori)
func NewProgress(totalUnits uint64, output io.Writer) *Progress {
	safeTotal := min(totalUnits, maxSafeUnits) // fall back to maxSafeUnits if totalUnits exceeds max precision

	terminalWidth := 80
	if f, ok := output.(*os.File); ok {
		terminalWidth = getWidth(f)
	}

	p := &Progress{
		output:     output,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
		resizeChan: make(chan os.Signal, 1),
		width:      max(terminalWidth, 80),
		clock:      &realClock{ dur: 16 * time.Millisecond },
	}
	p.total.Store(safeTotal)
	p.input.Store("")

	_ = p.write("\033[?25l") // hide the cursor

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

// write writes the provided data to p.output using the shared internal buffer to ensure an atomic system call.
// callers must acquire p.mu before calling this method to protect the shared internal buffer and to ensure UI consistency.
func (p *Progress) write(data ...any) error {
	p.buf = p.buf[:0]
	for _, v := range data {
		if s, ok := v.(string); ok {
			p.buf = append(p.buf, s...)
		} else {
			p.buf = fmt.Append(p.buf, v)
		}
	}
	_, err := p.output.Write(p.buf) // single, atomic system call (fmt.Fprintf makes no such guarantees)
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
	currentVal := p.current.Load()
	p.mu.Lock()
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

	var pct string // formatted percentage (unfortunately %3g%% doesn't quite work)
	switch {
	case percent >= 99.95:
		pct = "100"
	case percent >=  9.95:
		pct = fmt.Sprintf("%3.0f", percent)
	default:
		pct = fmt.Sprintf("%3.1f", percent)
	}

	prefix := fmt.Sprintf("processing (%s%%): ", pct)
	maxLen := max(p.width - len(prefix), 0)

	switch {
	case maxLen == 0:
		status = ""
	case len(status) > maxLen && maxLen > 3:
		status = "..." + status[len(status) - maxLen + 3:] // truncate from left to show most relevant portion (e.g., file basename)
	case len(status) > maxLen:
		status = status[:maxLen]
	}

	if width  == lastWidth &&
	   pct    == lastPct &&
	   status == lastStatus {
		return // skip redundant UI updates
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	err := p.write("\r\033[2K\r", prefix, status) // \033[2K clears the line, \r moves the cursor to the beginning of the line

	if err == nil {
		p.lastWidth  = width
		p.lastPct    = pct
		p.lastStatus = status
	}
}

// restoreAndExit restores the cursor upon trapping a SIGINT or SIGTERM signal.
func (p *Progress) restoreAndExit() {
	p.Close()
	os.Exit(1)
}

// Close stops the background renderer, restores the terminal cursor, and blocks until the final "done" state is displayed.
func (p *Progress) Close() {
	p.closeOnce.Do(func() {
		close(p.stopChan)                                          // stop the background renderLoop
		<-p.doneChan                                               // block until renderLoop exits
		p.mu.Lock()
		defer p.mu.Unlock()
		_ = p.write("\r\033[2K\rprocessing (100%): done\033[?25h") // render the final completion frame and restore the cursor
		if f, ok := p.output.(*os.File); ok {
			_ = f.Sync()                                           // immediately flush the final "done" message
		}
	})
}
