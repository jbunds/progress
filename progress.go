//go:build !test

// Package progress implements a simple progress indicator showing the status of processing.
package progress


import (
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/term"
)

// Progress holds the thread-safe state and synchronization channels for a throttled, single-line terminal progress bar.
type Progress struct {
	total    int64
	current  int64
	input    atomic.Value // stores the latest status, or the latest thing being processed
	stopChan chan struct{}
	doneChan chan struct{}
}

// NewProgress initializes a throttled, thread-safe progress indicator and begins the background terminal rendering loop.
func NewProgress(total int) *Progress {
	p := &Progress{
		total:    int64(total),
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
	p.input.Store("")

	fmt.Fprint(os.Stderr, "\033[?25l") // hide the cursor

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-sigChan:
			p.restoreAndExit()
		case <-p.stopChan:
			return // normal exit triggered by Close()
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

func (p *Progress) renderLoop() {
	ticker := time.NewTicker(16 * time.Millisecond) // ~60 FPS
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.draw()
		case <-p.stopChan:
			p.draw()
			fmt.Fprint(os.Stderr, "\033[?25h\n") // restore the cursor
			close(p.doneChan)
			return
		}
	}
}

func (p *Progress) draw() {
	cur     := atomic.LoadInt64(&p.current)
	percent := (cur * 100) / p.total

	status  := p.input.Load().(string)
	if percent >= 100 { status = "done" }

	width, _, err := term.GetSize(int(os.Stderr.Fd())) // #nosec G115
	if err != nil || width <= 0 { width = 80 }

	prefix  := fmt.Sprintf("\rprocessing (%3d%%): ", percent)
	maxLen  := width - len(prefix)
	
	display := status
	if len(display) > maxLen && maxLen > 3 {
		display = display[len(display)-maxLen+3:] + "..."
	}

	fmt.Fprintf(os.Stderr, "\033[2K%s%s", prefix, display) // \033[2K clears line, \r moves cursor to start
}

func (p *Progress) restoreAndExit() {
	fmt.Fprint(os.Stderr, "\033[?25h\n") // restore the cursor
	os.Exit(1)
}

// Close stops the background renderer, restores the terminal cursor, and blocks until the final "done" state is displayed.
func (p *Progress) Close() {
	close(p.stopChan)
	<-p.doneChan
}
