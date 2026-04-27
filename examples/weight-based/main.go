//go:build examples
package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"github.com/jbunds/progress"
)

type task struct {
	weight uint64
	delay  time.Duration
}

func main() {
	ctx  := context.Background()
	prog := progress.New(ctx, 0, os.Stderr)
	defer prog.Close(ctx)

	workload := randWork(100)       // 100 random tasks of varying weight

	for _, task := range workload { // workload discovery
		prog.AddTotal(task.weight)    // report to the progress tracker that a weighted task was discovered
	}

	for i, task := range workload { // workload processing
		time.Sleep(task.delay)        // simulate work
		prog.Report(float64(task.weight), fmt.Sprintf("task %d finished", i))
	}
}

func randWork(n int) []*task {
	tasks := make([]*task, n)
	for i := range n {
		tasks[i] = &task{
			weight: uint64(rand.IntN(50) + 1), // tasks have non-uniform weights ranging from 1 to 50
			delay:  rand.N(250 * time.Millisecond),
		}
	}
	return tasks
}
