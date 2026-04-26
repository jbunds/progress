//go:build examples
package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"github.com/jbunds/coverage/progress"
)

func main() {
	prog := progress.New(0, os.Stderr)
	defer prog.Close()

	totalBudget := prog.InitialBudget()
	simulateDiscovery(prog, "root", totalBudget, 0)
}

func simulateDiscovery(prog *progress.Progress, name string, budget float64, depth int) {
	if budget < 1e-6 { return } // floating point "zero" check
	if depth > 2 {
		processLeaf(prog, name, budget)
		return
	}

	numChildren := rand.IntN(4) + 2
	childShare  := budget / float64(numChildren)
	remaining   := budget

	for i := range numChildren {
		var currentShare float64
		if i == numChildren - 1 {
			currentShare = remaining
		} else {
			currentShare = childShare
			remaining   -= currentShare
		}

		childName := fmt.Sprintf("%s/node_%d", name, i)
		prog.Report(0, fmt.Sprintf("scanning %s...", childName))
		time.Sleep(200 * time.Millisecond)

		if rand.Float64() > 0.4 {
			simulateDiscovery(prog, childName, currentShare, depth+1)
		} else {
			processLeaf(prog, childName, currentShare)
		}
	}
}

func processLeaf(prog *progress.Progress, name string, budget float64) {
	time.Sleep(500 * time.Millisecond)
	prog.Report(budget, fmt.Sprintf("finished: %s", name))
}
