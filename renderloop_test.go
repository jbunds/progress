//go:build integration

package progress

import (
	"context"
	"io"
	"os"
	"runtime"
	"syscall"
	"testing"
	"testing/synctest"
	"time"
)

// TestRenderLoop_MemoryAllocRegression verifies that the unthrottled renderLoop
// and downstream trackers remain allocation-free during steady-state cycles.
//
// The test utilizes 'testing/synctest' to execute a high-volume workload inside a
// virtualized scheduler bubble, executing large iteration loops for each tracker.
//
// Regression detection thresholds are assessed proportionally (per operation)
// to mitigate background Go runtime housekeeping noise.
func TestRenderLoop_MemoryAllocRegression(t *testing.T) {
	taskCompleteMsg := "completed a task"

	benchmarks := []struct {
		name           string
		strategy       strategy
		totalWorkUnits uint64
	}{
		{ "Standard", Standard, 1e6 },
		{ "Unique",   Unique,   1e6 },
		{ "Fraction", Fraction, 1e6 },
		{ "Percent",  Percent,  1e6 },
	}

	for _, bm := range benchmarks {
		t.Run(bm.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				p := &Progress{
					tracker:        getTracker(bm.strategy, bm.totalWorkUnits),
					output:         io.Discard,
					tickerDuration: 16 * time.Millisecond,
					isTerminal:     func(any) bool { return true },
					resizeHandler:  func() int { return minWidth },
					stopChan:       make(chan struct{}),
					doneChan:       make(chan struct{}),
					// resizeChan is hijacked to trigger immediate renderLoop iterations by flooding the channel with SIGWINCH signals,
					// thus bypassing the 16ms ticker delay, and forcing the renderLoop goroutine to continuously execute draw cycles,
					// effectively transforming the time-throttled renderLoop into an unthrottled, CPU-bound process
					resizeChan:     make(chan os.Signal, bm.totalWorkUnits),
					theme:          newThemeRegistry().get("sunset"),
					fgColor:        fgColor(),
				}
				p.initBufPool()
				p.prepareTerminal()

				p.state.Store(uint32(256 & 0xFFFF) << 16)

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				t.Cleanup(func() { p.Close() })

				go p.renderLoop(ctx)

				synctest.Wait() // let renderLoop spin up and settle into its first select block
				runtime.GC()    // flush all allocations before gathering memory allocation metrics

				var memStart runtime.MemStats
				runtime.ReadMemStats(&memStart)

				for range bm.totalWorkUnits {
					p.Report(1, taskCompleteMsg)     // report the completion of a single work unit
					p.resizeChan <- syscall.SIGWINCH // trigger a window resize event to force a draw cycle
					// pause the main event loop and advance synctest's virtual clock until the renderLoop completes
					// a draw cycle and returns to a blocked state (i.e., awaits new [resize] events)
					synctest.Wait()
				}

				runtime.GC()
				var memEnd runtime.MemStats
				runtime.ReadMemStats(&memEnd)

				totalAllocs := memEnd.Mallocs    - memStart.Mallocs
				totalBytes  := memEnd.TotalAlloc - memStart.TotalAlloc
				allocsPerOp := float64(totalAllocs) / float64(bm.totalWorkUnits)
				bytesPerOp  := float64(totalBytes)  / float64(bm.totalWorkUnits)

				t.Logf("%s results -> total allocs: %d, allocs/op: %.6f, total bytes: %d, bytes/op: %.6f",
					bm.name, totalAllocs, allocsPerOp, totalBytes, bytesPerOp)

				if allocsPerOp > 0.0001 { // a real memory leak will scale lineraly with bm.totalWorkUnits
					t.Errorf("%s memory regression: expected < 0.0001 allocs/op, got %.6f (total allocs: %d)", bm.name, allocsPerOp, totalAllocs)
				}

				if bytesPerOp > 0.1 { // a readl memory leak will scale lineraly with bm.totalWorkUnits
					t.Errorf("%s memory regression: expected < 0.1 bytes/op, got %.6f (total bytes: %d)", bm.name, bytesPerOp, totalBytes)
				}

				p.Close()
				synctest.Wait() // ensure cleanup actions are performed
			})
		})
	}
}
