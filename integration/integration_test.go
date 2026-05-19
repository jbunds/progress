//go:build integration

package integration

import (
  "context"
  "math/rand/v2"
  "os"
  "os/signal"
	"strconv"
  "syscall"
	"testing"
  "time"

  "github.com/jbunds/progress"
)

const (
  // case a (default): 100 tasks, standard weights
  totalTasks uint64 = 100
  maxWeight  uint64 = 50

  // case b (massive individual items): 100 tasks, extreme weights
  // totalTasks uint64 = 100
  // maxWeight  uint64 = 1e12

  // case c (high-frequency stress test): 10 billion items, light weights
  // totalTasks uint64 = 1e10 
  // maxWeight  uint64 = 50   
)

func TestStreamingStress(t *testing.T) {
  ctx, stop := signal.NotifyContext(context.Background(),
    os.Interrupt,    // interrupt signal (ctrl+c)
		syscall.SIGTERM, // kill signal
		syscall.SIGHUP)  // terminal closed signal
  defer stop()

  prog := progress.New(ctx, 0, os.Stderr, progress.WithIsTerminalFunc(func(any) bool { return true }))
//	t.Cleanup(func() { prog.Close() })

  // phase 1: stream workload discovery
  var discoveredWeight uint64
	for i := range totalTasks { // simulate discovering the total weight upfront without holding slice items in memory
		if i % 1e7 == 0 {         // check for canceled context every 1e7 iterations in case the operator sent one of the signals we listen for
			if err := ctx.Err(); err != nil {
				t.Logf("aborted (%v)", err.Error())
				return 
			}
		}
    weight           := uint64(rand.Uint64N(maxWeight) + 1) // #nosec G404
    discoveredWeight += weight
  }
  prog.AddTotal(discoveredWeight)

  // phase 2: stream workload processing
	for i := range totalTasks { // process tasks lazily on the fly. memory footprint should remain low, ~10-20 MB
    select {
    case <-ctx.Done():
      return
    default:
      weight := uint64(rand.Uint64N(maxWeight) + 1) // #nosec G404
      
      // throttle delays only if running a small, readable task set
      if totalTasks <= 100 {
        time.Sleep(rand.N(250 * time.Millisecond)) // #nosec G404
      }

      // log status string samples dynamically on a strict cadence to prevent terminal spam
      if totalTasks <= 100 || i % (totalTasks / 10) == 0 {
        prog.Report(float64(weight), "processed stream chunk " + strconv.FormatUint(i, 10))
      } else {
        prog.Report(float64(weight), "") // pure numeric updates on the hot-path
      }
    }
  }
	prog.Close()
}
