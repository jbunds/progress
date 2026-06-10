//go:build examples

// smoke test program to validate the following functionality:
//
//   - status strings longer than the width of the terminal are correctly truncated
//
//   - the progress bar extends to the full width of the terminal at 100% progress
//     by configuring a 1:1 column-to-task ratio
//
//   - the dynamically-computed foreground color applied to the status text rendered
//     in the progress bar contrasts well against the background color
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jbunds/progress"
	"github.com/jbunds/progress/examples"
	"golang.org/x/term"
)

func main() {
	exitCode := 0
	defer func() { os.Exit(exitCode) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		os.Interrupt,    // SIGINT  ( 2)
		syscall.SIGQUIT, // SIGQUIT ( 3)
		syscall.SIGTERM, // SIGTERM (15)
		syscall.SIGHUP)  // SIGHUP  ( 1)

	var receivedSignal os.Signal
	go func() {
		receivedSignal = <-sigChan
		signal.Stop(sigChan)
		cancel() // trigger ctx.Done()
	}()

	output          := os.Stderr
	termWidth, _, _ := term.GetSize(int(output.Fd()))
	if termWidth < 80 { termWidth = 80 } // fallback when piped or redirected
	tasks           := uint64(termWidth)
	status          := "blah blah blah "
	repeatCount     := termWidth / len(status) // len("task 1 finished ") == 16 > len("blah blah blah ") == 15

	prog := progress.New(ctx, tasks, output, examples.Flags(flag.CommandLine, os.Args[1:])...)
	defer prog.Close()

	for i := range tasks {
		select {
		case <-ctx.Done():
			if receivedSignal != nil {
				if sig, ok := receivedSignal.(syscall.Signal); ok {
					exitCode = int(sig) + 128
				}
			}
			return
		case <-time.After(18 * time.Millisecond):
			prog.Report(1, fmt.Sprintf("task %d finished " + strings.Repeat(status, repeatCount), i))
		}
	}
}
