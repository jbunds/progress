// Package progress provides status updates for units of work being concurrently processed.
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
	// (sync/atomic provides no floating-point types)
	//
	// the choice of 1e12 balances high-precision fractional shares in the context of deep recursion with sufficient
	// uint64 headroom to prevent overflow when performing intermediate percentage calculations (currentVal * 100)
	scale        uint64 = 1e12
	// maxSafeUnits is the maximum number of work units allowed before intermediate percentage
	// calculations risk uint64 overflow; some precision will be lost when totalUnits > maxSafeUnits
	maxSafeUnits uint64 = math.MaxUint64 / scale
)

// Progress provides a concurrency-safe, high-precision status indicator for both fixed-batch and recursive workloads.
type Progress struct {
	total      uint64        // 0 for fractional path allocation; > 0 for weight-based accumulation
	current    atomic.Uint64 // accumulates shares of scale
	input      atomic.Value  // stores the latest unit of work being processed
	stopChan   chan struct{} // signals the background rendering loop to perform final cleanup and exit
	doneChan   chan struct{} // doneChan is closed once the rendering loop has finished its final draw and cursor restoration
	output     io.Writer     // destination writer for the terminal-formatted work progress status updates
	clock      clock         // provides the timing source for throttled UI updates, allowing for fake clocks in tests
	width      int           // width of the terminal window (set during construction, so resizing of the terminal window at runtime is not properly handled)
	drawNotify chan struct{} // optional channel used to signal completion of a draw cycle for deterministic testing
	drawnDone  atomic.Bool   // drawnDone ensures the final completion frame is rendered just once to prevent status smearing
	closeOnce  sync.Once     // closeOnce ensures that cursor restoration and cleanup logic are executed just once
}

// obviates, e.g., sleeping in unit tests
type clock interface { tick() <-chan time.Time }

type realClock struct { dur time.Duration }
func (r *realClock) tick() <-chan time.Time { return time.NewTicker(r.dur).C }

type fakeClock struct { chn chan time.Time }
func (f *fakeClock) tick() <-chan time.Time { return f.chn }

// NewProgress initializes a throttled, concurrency-safe work unit completion (progress)
// tracker and starts a terminal work progress status rendering loop in the background.
//
//    pass totalUnits >  0 for weight-based accumulation  (when totalUnits is known a priori)
//    pass totalUnits == 0 for fractional path allocation (when totalUnits is not known a priori)
func NewProgress(totalUnits uint64, output io.Writer) *Progress {
	safeTotal := totalUnits
	if totalUnits > maxSafeUnits {
		fmt.Fprintf(os.Stderr, "totalUnits %d exceeds max precision; falling back to %d\n", totalUnits, maxSafeUnits)
		safeTotal = maxSafeUnits
	}
	w := 80
	if f, ok := output.(*os.File); ok {
		w = getWidth(f) // output is a real *os.File (and not, e.g., io.Discard, as the case may be for certain tests)
	}

	p := &Progress{
		total:    safeTotal,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
		output:   output,
		width:    max(w, 80),
		clock:    &realClock{ dur: 16 * time.Millisecond },
	}

	p.input.Store("")

	fmt.Fprint(p.output, "\033[?25l") // hide the cursor

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM) // trap SIGINT and SIGTERM so the hidden cursor can be restored

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
		// although f.Fd() will certainly be 0 (os.Stdin), 1 (os.Stdout), or 2 (os.Stderr),
		// the following check is performed to satisfy the gosec linter (otherwise
		// gosec complains about possible integer overflow per the call to int())
		if fd > math.MaxInt { continue } // skip if FD is logically impossible for term.GetSize (really, just making gosec happy)
		if w, _, err := term.GetSize(int(fd)); err == nil {
			if w > width { width = w }
		}
	}
	return max(width, 80) // fallback for pipes, redirects, and non-tty outputs
}

// InitialBudget returns the full internal scale (100%) to be used as the starting budget for tracking fractional progress.
func (p *Progress) InitialBudget() uint64 { return scale }

// Report records the completion of a unit of work, or a fractional share or work, and updates the status message.
//
//   if total >  0: 'val' is the weight of the completed unit
//   if total == 0: 'val' is the scaled budget (portion of scale) for the branch
func (p *Progress) Report(val uint64, status string) {
	if status != "" { p.input.Store(status) }
	var share uint64
	if p.total > 0 {
		safeVal := min(val, p.total)
		share = (safeVal * scale) / p.total // weight-based accumulation mode: calculate share of scale
	} else {
		share = val                         // fractional path allocation mode: add the budget share directly
	}
	p.current.Add(share)
}

// renderLoop periodically draws the progress line at ~60 FPS to ensure a smooth UI without impeding the processing logic.
func (p *Progress) renderLoop() {
	tickerChan := p.clock.tick()
	for {
		select {
		case <-tickerChan:
			p.draw()
		case <-p.stopChan:
			p.draw()
			fmt.Fprint(p.output, "\033[2K\r") // clear the progress status line
			fmt.Fprint(p.output, "\033[?25h") // restore the cursor
			close(p.doneChan)
			return
		}
	}
}

// draw clears the current terminal line and prints the formatted percentage and status string, truncating text as needed to fit within the terminal width.
func (p *Progress) draw() {
	currentVal := p.current.Load()
	safeVal    := min(currentVal, scale)  // prevent uint64 overflow in percentage calculations, and ensure the UI never reports > 100%
	percent    := (safeVal * 100) / scale // multiply before dividing for precision; safe from uint64 overflow when currentVal <= ~1.8e17

	status, _  := p.input.Load().(string)
	if percent >= 100 { status = "done" }

	defer func() {
		if p.drawNotify != nil { // enables deterministic tests
			p.drawNotify <- struct{}{}
		}
	}()

	if percent == 100 && status == "done" {
		if p.drawnDone.Swap(true) { return }
	}

	prefix  := fmt.Sprintf("processing (%3d%%): ", percent)
	maxLen  := max(p.width - len(prefix), 0)
	display := status

	switch {
	case maxLen == 0:
		display = ""
	case len(display) > maxLen && maxLen > 3:
		display = "..." + display[len(display)-maxLen+3:] // truncate from left to show most relevant portion (e.g., file basename)
	case len(display) > maxLen:
		display = display[:maxLen]
	}

	fmt.Fprintf(p.output, "\r\033[2K%s%s", prefix, display) // \033[2K clears the line, \r moves the cursor to the beginning of the line
}

// restoreAndExit restores the cursor upon trapping a SIGINT or SIGTERM signal.
func (p *Progress) restoreAndExit() {
	fmt.Fprint(p.output, "\033[?25h\n") // restore the cursor
	os.Exit(1)
}

// Close stops the background renderer, restores the terminal cursor, and blocks until the final "done" state is displayed.
func (p *Progress) Close() {
	p.closeOnce.Do(func() {
		p.current.Store(scale) // force internal counter to 100%
		p.input.Store("done")  // force status to "done"
		close(p.stopChan)
		<-p.doneChan
	})
}
