//go:build examples
package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jbunds/progress"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,    // interrupt signal (ctrl+c)
		syscall.SIGTERM, // kill signal
		syscall.SIGHUP)  // terminal closed signal
	defer stop()

	prog := progress.New(ctx, 0, os.Stderr)
	defer prog.Close()

	simulateDiscovery(ctx, prog, "root", prog.InitialBudget(), 0)
}

func simulateDiscovery(ctx context.Context, prog *progress.Progress, name string, budget float64, depth int) {
	if budget < 1e-6 { return } // floating point "zero" check
	if depth > 2 {
		processLeaf(ctx, prog, name, budget)
		return
	}

	numChildren := rand.IntN(4) + 2
	childShare  := budget / float64(numChildren)
	remaining   := budget

	for i := range numChildren {
		childName := fmt.Sprintf("%s/node_%d", name, i)
		prog.Report(0, fmt.Sprintf("scanning %s...", childName))

		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}

		var currentShare float64
		if i == numChildren - 1 {
			currentShare = remaining
		} else {
			currentShare = childShare
			remaining   -= currentShare
		}

		if rand.Float64() > 0.4 {
			simulateDiscovery(ctx, prog, childName, currentShare, depth + 1)
		} else {
			processLeaf(ctx, prog, childName, currentShare)
		}
	}
}

func processLeaf(ctx context.Context, prog *progress.Progress, name string, budget float64) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(500 * time.Millisecond):
		prog.Report(budget, fmt.Sprintf("finished: %s", name))
	}
}
