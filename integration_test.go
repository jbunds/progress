//go:build integration

package progress

import (
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"
)

// system info:
//
//   $ system_profiler SPHardwareDataType 2>/dev/null | egrep '^\s+(?:Model|Chip|Total|Memory)' | column -s: -t
//         Model Name              MacBook Air
//         Model Identifier        MacBookAir10,1
//         Model Number            MGN73SM/A
//         Chip                    Apple M1
//         Total Number of Cores   8 (4 Performance and 4 Efficiency)
//         Memory                  8 GB
//
// test results:
//
//   $ ./integration_test.sh
//   args: -totaltasks  100 -loopiterations 100
//   time:    0.38s
//   mem:     4.4078 MB
//
//   args: -totaltasks 1000 -loopiterations 1e7
//   time:    8.10s
//   mem:     1519.3950 MB
//
//   args: -totaltasks  1e6 -loopiterations 1e6
//   time:    0.63s
//   mem:     157.5956 MB
//
//   args: -totaltasks    0 -loopiterations 100
//   time:    0.00s
//   mem:     4.1265 MB
//
//   args: -totaltasks    0 -loopiterations 1e6
//   time:    0.63s
//   mem:     166.6425 MB
//
//   args: -totaltasks    0 -loopiterations 1e7
//   time:    7.79s
//   mem:     1449.8793 MB

// digits is used by the nextUniqueString helper function to generate status
// strings unique to a worker goroutine to observe heap allocations by
// the SUT while it's being bombarded with concurrent calls to Report().
var digits = []byte("0123456789abcdef")

// TestStreamingStress is quick-and-dirty smoke / stress test designed to reveal the heap allocation
// behavior of the SUT as it's being bombarded with concurrent calls to Report() by workers.
//
// Report() serves as the entry point for the downstream call stack which exercises the core
// functionality of the SUT.
//
// The BenchmarkRenderLoop benchmark test defined in progress_bench_test.go
// is designed to provide less noisy and more precise heap allocation metrics.
func TestStreamingStress(t *testing.T) {
	totalTasks, loopIterations := params(flag.CommandLine, filterArgs(os.Args[1:]))

	timeChan := make(chan time.Time, 5000) // high-capacity buffer to handle concurrent worker status updates
	
	prog := New(t.Context(), totalTasks, io.Discard,
		WithIsTerminalFunc(func(any) bool { return true }), // force ANSI sequence-encoded rendering
		withClock(fakeClock{c: timeChan}))

	const workerCount = 3
	iterationsPerWorker := loopIterations / workerCount

	var wg sync.WaitGroup

	for w := range workerCount {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			localRand := rand.New(rand.NewPCG(rand.Uint64(), uint64(workerID))) // fast local, non-crypto math/rand source to bypass the global CSPRNG lock
			startIdx  := workerID * int(iterationsPerWorker)
			endIdx    := startIdx + int(iterationsPerWorker)
			taskID    := []byte("worker chunk 0000000000000000")    // 16 digits for uint64 space

			for i := startIdx; i < endIdx; i++ {
				taskCompleteMsg := nextUniqueString(taskID, i)        // causes heap allocs to explode in proportion to loop iterations
				if totalTasks == 0 {                                  // fractional path allocation API mode (dynamic task discovery)
					taskSize := localRand.Uint64N(50) + 1
					if localRand.Uint64N(20) == 0 {                     // interleave concurrent task discovery with 5% probability trigger to stress test atomic operations
						taskSize = localRand.Uint64N(100) + 1             // simulate variable weight tasks
						prog.AddTotal(taskSize)                           // simulate variable task size ranging from 1 to 100 units
					}
					prog.Report(float64(taskSize), taskCompleteMsg)
				} else {                                              // weight-based accumulation API mode (fixed number of tasks)
					currentWeight := float64(localRand.Uint64N(50) + 1) // simulate variable weight tasks
					prog.Report(currentWeight, taskCompleteMsg)
				}

				select { // notify renderLoop without deadlocking workers
				case timeChan <- time.Time{}:
				default: // buffer saturated; drop the frame tick to prevent worker starvation
				}
			}
		}(w)
	}

	wg.Wait()

	prog.Close()

	select {
	case <-prog.doneChan: // blocks until the renderLoop goroutine exits
	case <-time.After(2 * time.Second):
		t.Fatal("test timed out waiting for renderLoop to finalize")
	}
}

// nextUniqueString mutates the passed byte array in-place to generate a new string
// unique among the set of calls to Report() within its parent worker goroutine.
func nextUniqueString(buf []byte, val int) string {
	pos := len(buf) - 1 // mutate the numeric suffix of the buffer from right to left
	for range 16 {
		buf[pos] = digits[val & 0xF]
		val >>= 4
		pos--
	}
	return unsafe.String(&buf[0], len(buf)) // zero-alloc conversion
}

// params parses the -totaltasks and -loopiterations command line flag parameters used to define the runtime bounds of the test.
// flag values must be positive integers, optionally expressed in scientific notation.
func params(fs *flag.FlagSet, args []string) (totalTasks, loopIterations uint64) {
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "%s usage:\n\n", filepath.Base(fs.Name()))
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output())
	}
	totalTasksVal     := sciUint64(  0)
	loopIterationsVal := sciUint64(100)
	fs.Var(&totalTasksVal,     "totaltasks",     "total tasks (0 triggers fractional mode)")
	fs.Var(&loopIterationsVal, "loopiterations", "total loop iterations to simulate work processing")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "%v: falling back to defaults\n", err)
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintf(fs.Output(), "ignored arguments: %s\n", strings.Join(fs.Args(), ", "))
	}
	return uint64(totalTasksVal), uint64(loopIterationsVal)
}

// sciUint64 represents a uint64 optionally expressed in scientific notation
// to allow command line flag arguments to be parsed accordingly.
type sciUint64 uint64

func (s *sciUint64) String() string { return strconv.FormatUint(uint64(*s), 10) }

func (s *sciUint64) Set(value string) error {
	val, err := strconv.ParseFloat(value, 64)
	if err != nil { return fmt.Errorf("invalid numeric value %q: %w", value, err) }
	if val <    0 { return fmt.Errorf("value %q cannot be negative",  value)      }
	*s = sciUint64(val)
	return nil
}

func filterArgs(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			args = args[i + 1:]
			break
		}
	}
	return args
}
