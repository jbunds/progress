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
	prog := progress.NewProgress(0, os.Stderr)
	defer prog.Close()

	totalBudget := prog.InitialBudget()
	
	simulateDiscovery(prog, "root", totalBudget, 0)
}

func simulateDiscovery(p *progress.Progress, name string, budget uint64, depth int) {
	if budget == 0 { return }

	if depth > 2 {
		processLeaf(p, name, budget)
		return
	}

	numChildren := rand.IntN(4) + 2 // a task can have between 2 and 5 sub-tasks
	
	childShare := budget / uint64(numChildren) // base share among children
	remainder  := budget % uint64(numChildren) // remainder

	for i := range numChildren {
		currentShare := childShare
		if i == numChildren - 1 {
			currentShare += remainder // grant the remainder to the last child
		}

		childName := fmt.Sprintf("%s/node_%d", name, i)
		
		p.Report(0, fmt.Sprintf("scanning %s...", childName))
		time.Sleep(300 * time.Millisecond) // simulate some discovery time

		if rand.Float64() > 0.4 { // pseudorandomly dive deeper or finish by processing a leaf node
			simulateDiscovery(p, childName, currentShare, depth + 1)
		} else {
			processLeaf(p, childName, currentShare)
		}
	}
}

func processLeaf(p *progress.Progress, name string, budget uint64) {
	time.Sleep(500 * time.Millisecond) // simulate the work of processing a file
	p.Report(budget, fmt.Sprintf("finished: %s", name))
}
