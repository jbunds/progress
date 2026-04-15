// Package progress implements a simple progress indicator showing the status of processing.
package progress

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/term"
)

var initialStderrFd uintptr

func init() { initialStderrFd = os.Stderr.Fd() }

// Progress holds the thread-safe state and synchronization channels for a throttled, single-line terminal progress bar.
type Progress struct {
	total      int64
	current    int64
	input      atomic.Value // stores the latest status, or the latest thing being processed
	stopChan   chan struct{}
	doneChan   chan struct{}
	output     io.Writer
	clock      clock
	drawNotify chan struct{}
	fd         uintptr
}

// obviates sleeping in unit tests
type clock interface { tick() <-chan time.Time }

type realClock struct { d time.Duration }
func (r *realClock) tick() <-chan time.Time { return time.NewTicker(r.d).C }

type fakeClock struct { c chan time.Time }
func (f *fakeClock) tick() <-chan time.Time { return f.c }

// NewProgress initializes a throttled, thread-safe progress indicator and begins the background terminal rendering loop.
func NewProgress(total int, output io.Writer) *Progress {
	p := &Progress{
		total:    int64(total),
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
		output:   output,
		clock:    &realClock{ d: 16 * time.Millisecond },
	}

	if f, ok := output.(*os.File); ok {
		p.fd = f.Fd()
	} else {
		p.fd = initialStderrFd
	}

	p.input.Store("")

	fmt.Fprint(p.output, "\033[?25l") // hide the cursor

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		defer signal.Stop(sigChan) // clean up signal listener
		select {
		case <-sigChan:
			p.restoreAndExit()     // SIGINT or SIGTERM trapped; restore the hidden cursor
		case <-p.stopChan:
			return                 // normal exit triggered by Close()
		}
	}()

	go p.renderLoop()
	return p
}

// Update updates the progress status indicator.
func (p *Progress) Update(input string) {
	atomic.AddInt64(&p.current, 1)
	p.input.Store(input)
}

// renderLoop periodically draws the progress line at ~60 FPS to ensure a smooth UI without bottlenecking the processing logic.
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

// draw clears the current terminal line and prints the formatted percentage and status string, truncating text as needed to fit the terminal width.
func (p *Progress) draw() {
	cur     := atomic.LoadInt64(&p.current)
	percent := (cur * 100) / p.total

	status, _ := p.input.Load().(string)
	if percent >= 100 { status = "done" }

	width, _, err := term.GetSize(int(p.fd)) // #nosec G115
	if err != nil || width <= 0 { width = 80 }

	prefix  := fmt.Sprintf("processing (%3d%%): ", percent)
	maxLen  := width - len(prefix)
	
	display := status
	if len(display) > maxLen && maxLen > 3 {
		display = display[len(display)-maxLen+3:] + "..."
	}

	fmt.Fprintf(p.output, "\r\033[2K%s%s", prefix, display) // \033[2K clears the line, \r moves the cursor to the beginning of the line

	if p.drawNotify != nil { p.drawNotify <- struct{}{} }   // enables deterministic tests
}

// restoreAndExit restores the cursor upon trapping a SIGINT or SIGTERM signal.
func (p *Progress) restoreAndExit() {
	fmt.Fprint(p.output, "\033[?25h\n") // restore the cursor
	os.Exit(1)
}

// Close stops the background renderer, restores the terminal cursor, and blocks until the final "done" state is displayed.
func (p *Progress) Close() {
	close(p.stopChan)
	<-p.doneChan
}
