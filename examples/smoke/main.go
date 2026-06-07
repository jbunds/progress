//go:build examples

// smoke test program to validate the following functionality:
//
//   1. status strings longer than the width of the terminal are correctly truncated
//   2. the progress bar extends to the full width of the terminal at 100% progress by configuring a 1:1 column-to-task ratio
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
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,    // interrupt signal (ctrl+c)
		syscall.SIGTERM, // kill signal
		syscall.SIGHUP)  // terminal closed signal
	defer stop()

	output          := os.Stderr
	termWidth, _, _ := term.GetSize(int(output.Fd()))
	if termWidth <= 0 { termWidth = 80 } // fallback when piped or redirected
	tasks           := uint64(max(termWidth, 0))
	status          := "blah blah blah "
	repeatCount     := termWidth / len(status) // len("task 1 finished ") == 16 > len("blah blah blah ") == 15

	prog := progress.New(ctx, tasks, output, examples.Flags(flag.CommandLine, os.Args[1:])...)
	defer prog.Close()

	for i := range tasks {
		select {
		case <-ctx.Done():
			return
		case <-time.After(25 * time.Millisecond):
			prog.Report(1, fmt.Sprintf("task %d finished " + strings.Repeat(status, repeatCount), i))
		}
	}
}
