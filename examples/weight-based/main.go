//go:build examples

// example program demonstrating the progress package API.
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

type task struct {
	weight uint64
	delay  time.Duration
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,    // interrupt signal (ctrl+c)
		syscall.SIGTERM, // kill signal
		syscall.SIGHUP)  // terminal closed signal
	defer stop()

	prog := progress.New(ctx, 0, os.Stderr)
	defer prog.Close()

	workload := randomWork(100)        // 100 random tasks of varying weight

	for _, task := range workload {    // workload discovery
		prog.AddTotal(task.weight)     // report to the progress tracker that a weighted task was discovered
	}

	for i, task := range workload {    // workload processing
		select {
		case <-ctx.Done():
			return
		case <-time.After(task.delay): // simulate work
			prog.Report(float64(task.weight), fmt.Sprintf("task %d finished", i))
		}
	}
}

func randomWork(n int) []*task {
	tasks := make([]*task, n)
	for i := range n {
		tasks[i] = &task{ // tasks have non-uniform weights ranging from 1 to 50
			weight: rand.Uint64N(50) + 1,           // #nosec G404
			delay:  rand.N(250 * time.Millisecond), // #nosec G404
		}
	}
	return tasks
}
