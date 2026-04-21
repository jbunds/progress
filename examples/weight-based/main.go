//go:build examples
package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"github.com/jbunds/coverage/progress"
)

type task struct {
	weight uint64
	delay  time.Duration
}

func main() {
	totalUnits := uint64(1e3)
	prog       := progress.NewProgress(totalUnits, os.Stderr)
	defer prog.Close()
	for i, t := range randWork(totalUnits) {
		time.Sleep(t.delay)
		prog.Report(t.weight, fmt.Sprintf("work unit %d of %d completed", i, totalUnits))
	}
}

func randWork(n uint64) []*task {
	tasks := make([]*task, n)
	for i := range n {
		tasks[i] = &task{
			weight: 1,
			delay:  rand.N(25 * time.Millisecond),
		}
	}
	return tasks
}
