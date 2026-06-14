package progress

import (
	"io"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// $ go test -run='^$' -memprofile=mem.pprof -benchtime=5s -bench=./...
// goos: darwin
// goarch: arm64
// pkg: github.com/jbunds/progress
// cpu: Apple M1
// BenchmarkRenderLoop/Standard/throughput-8           13541190         395.8 ns/op         0 B/op        0 allocs/op
// BenchmarkRenderLoop/Standard/isolated_sample-8      13976658         431.2 ns/op         0 B/op        0 allocs/op
// --- BENCH: BenchmarkRenderLoop/Standard/isolated_sample-8
//     progress_bench_test.go:220: total memory allocated: 320 bytes (0.31 kB)
// BenchmarkRenderLoop/Unique/throughput-8             15652609         380.2 ns/op         0 B/op        0 allocs/op
// BenchmarkRenderLoop/Unique/isolated_sample-8        13769311         437.1 ns/op         0 B/op        0 allocs/op
// --- BENCH: BenchmarkRenderLoop/Unique/isolated_sample-8
//     progress_bench_test.go:220: total memory allocated: 208 bytes (0.20 kB)
// BenchmarkRenderLoop/Fraction/throughput-8           22029908         274.1 ns/op         0 B/op        0 allocs/op
// BenchmarkRenderLoop/Fraction/isolated_sample-8      18032866         332.5 ns/op         0 B/op        0 allocs/op
// --- BENCH: BenchmarkRenderLoop/Fraction/isolated_sample-8
//     progress_bench_test.go:220: total memory allocated: 128 bytes (0.12 kB)
// BenchmarkRenderLoop/Percent/throughput-8            25747536         233.1 ns/op         0 B/op        0 allocs/op
// BenchmarkRenderLoop/Percent/isolated_sample-8       20289648         299.7 ns/op         0 B/op        0 allocs/op
// --- BENCH: BenchmarkRenderLoop/Percent/isolated_sample-8
//     progress_bench_test.go:220: total memory allocated: 32 bytes (0.03 kB)
// PASS
// ok    github.com/jbunds/progress  47.691s

// see also `go build -gcflags=-m`

// never pollute benchmark profile data with allocations triggered by unit tests
// (e.g., TestReportContention's strconv.Itoa() and strconv.Atoi() calls)
//
// always run benchmark profiling suites in isolation by passing
// `-run='^$'` to instruct `go test` to skip all unit tests
//
// continuously execute the renderLoop for 10 seconds, scaling b.N proportionally:
//
//   go test -run='^$' -memprofile=mem.pprof -benchtime=10s -bench=./...
//
// execute all benchmark tests 10 times:
//
//   go test -run='^$' -memprofile=mem.pprof -count=10 -bench=./...
//
// execute 5 million renderLoop iterations (b.N == 5e6):
//
//   go test -run='^$' -memprofile=mem.pprof -benchtime=5000000x -bench=./...
//
// inspect the memory allocation results written to mem.pprof
// for all functions or methods in the "progress" package:
//
//   go tool pprof progress.test mem.pprof
//   go tool pprof -alloc_objects mem.pprof  # shows every object ever created during the benchmark test run
//   go tool pprof -inuse_objects mem.pprof  # shows real memory leaks because the GC cannot free those items
//
//   (pprof) list progress
//
//   (pprof) focus=Report  # emits null output
//
//   sample_index=inuse_space:   shows bytes currently allocated (best for finding large leaked objects)
//   sample_index=inuse_objects: shows count of objects currently allocated (best for finding loops/growing slices leaking small objects)
//   sample_index=alloc_space:   shows total bytes allocated since the program started
//
//   top20:                      lists the top 20 functions holding onto memory. Look for unexpected custom packages at the top
//   top -cum:                   sorts functions by cumulative memory (the function itself plus all functions it called). Excellent for tracing the execution path
//   list <functionName>:        shows line-by-line memory allocation for a specific function. This will tell you the exact line of code causing the leak
//
// profile targets: filter using pprof focus / hide directives as required
//
// because pprof indexes only those symbols which actually allocate memory onto
// the heap during the profiling window (between b.ResetTimer() and b.StopTimer()),
// a null result from any queries specifying symbols unique to the "progress"
// package definitively indicates that no objects were allocated onto the heap
// by those symbols during the run
//
// view the profile via a web UI:
//
//   go tool pprof -http=:8080 mem.pprof

// interrogate the Go compiler to reveal its escape analysis optimization decisions:
//
//   go test -gcflags='-l -m' ./... | fgrep progress.go
//
// -l: disables inlining to produce more readable output
// -m: shows optimization decisions including escapes to the heap

func BenchmarkRenderLoop(b *testing.B) {
	// set storeLastFrameHook (used by unit tests) to a no-op function since it allocates onto the heap
	storeLastFrameHook = func(*Progress, []byte) {}

	taskCompleteMsg := "completed a task" // pre-allocated variable to eliminate interface / string literal heap escapes

	benchmarks := []struct {
		name           string
		strategy       strategy
		totalWorkUnits uint64
	}{
		{ "Standard", Standard, 1e8 },
		{ "Unique",   Unique,   1e8 },
		{ "Fraction", Fraction, 1e8 },
		{ "Percent",  Percent,  1e8 },
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {

			// sub-benchmark 1: profile real-world renderLoop throughput
			b.Run("throughput", func(subB *testing.B) {
				p := &Progress{
					tracker:        getTracker(bm.strategy, bm.totalWorkUnits),
					output:         io.Discard,
					tickerDuration: 16 * time.Millisecond,
					isTerminal:     func(any) bool { return true }, // force ANSI sequence encoding
					resizeHandler:  func() int { return minWidth },
					stopChan:       make(chan struct{}),
					doneChan:       make(chan struct{}),
					// resizeChan is hijacked to trigger immediate renderLoop iterations by flooding the channel with SIGWINCH signals,
					// thus bypassing the 16ms ticker delay, and forcing the renderLoop goroutine to continuously execute draw cycles,
					// effectively transforming the time-throttled renderLoop into an unthrottled, CPU-bound process
					resizeChan:     make(chan os.Signal, 1),
				}
				p.initBufPool()
				p.prepareTerminal()

				p.state.Store(uint32(256 & 0xFFFF) << 16)

				go p.renderLoop(subB.Context())
				subB.Cleanup(func() { p.Close() })

				subB.ResetTimer()
				subB.ReportAllocs()

				for subB.Loop() {
					p.Report(10, taskCompleteMsg)
					select {
					case p.resizeChan <- syscall.SIGWINCH: // trigger a window resize event to force a draw cycle
					default:
					}
					runtime.Gosched() // yield the scheduler to allow the renderLoop goroutine to flush its channel buffer
				}

				subB.StopTimer()
			})

			// sub-benchmark 2: isolated zero-alloc test guard using real-world memory profiles
			b.Run("isolated sample", func(subB *testing.B) {
				p := &Progress{
					tracker:        getTracker(bm.strategy, bm.totalWorkUnits),
					output:         io.Discard,
					tickerDuration: 16 * time.Millisecond,
					isTerminal:     func(any) bool { return true }, // force ANSI sequence encoding
					resizeHandler:  func() int { return minWidth },
					stopChan:       make(chan struct{}),
					doneChan:       make(chan struct{}),
					// resizeChan is hijacked to trigger immediate renderLoop iterations by flooding the channel with SIGWINCH signals,
					// thus bypassing the 16ms ticker delay, and forcing the renderLoop goroutine to continuously execute draw cycles,
					// effectively transforming the time-throttled renderLoop into an unthrottled, CPU-bound process
					resizeChan:     make(chan os.Signal, 1),
				}
				p.initBufPool()
				p.prepareTerminal()

				p.state.Store(uint32(256 & 0xFFFF) << 16)

				go p.renderLoop(subB.Context())
				subB.Cleanup(func() { p.Close() })

				subB.ResetTimer()
				subB.ReportAllocs()
				subB.ReportMetric(0, "allocs/op") // clear default metric display

				var memBefore, memAfter runtime.MemStats
				runtime.GC() // clean up the heap before capturing baseline
				runtime.ReadMemStats(&memBefore)

				var iterations uint64
				for subB.Loop() {
					// change the following statement to:
					//
					//   testP.Report(10, "task " + strconv.Itoa(subB.N))
					//
					// to observe:
					//
					//   allocs/op == 1
					//    bytes/op == 8
					p.Report(10, taskCompleteMsg)
					select {
					case p.resizeChan <- syscall.SIGWINCH: // trigger a window resize event to force a draw cycle
					default:
					}
					runtime.Gosched() // yield the scheduler to allow the renderLoop goroutine to flush its channel buffer
					iterations++
				}

				subB.StopTimer()
				runtime.ReadMemStats(&memAfter)

				// TODO(jeff): consider something like:
				//
				//   func TestMemoryLeak(t *testing.T) { result := testing.Benchmark(func(b *testing.B) { ... } }
				//
				// to inspect the BenchmarkResult:
				//
				//   bytesPerOp  := result.AllocedBytesPerOp()
				//   allocsPerOp := result.AllocsPerOp()
				totalBytesAlloced   := memAfter.TotalAlloc - memBefore.TotalAlloc
				totalObjectsAlloced := memAfter.Mallocs    - memBefore.Mallocs
				bytesPerOp          := totalBytesAlloced   / iterations
				allocsPerOp         := totalObjectsAlloced / iterations

				remediationAction := "run `go test -run='^$' -memprofile=mem.pprof -bench=BenchmarkRenderLoop` to isolate the memory leak"

				subB.Logf("total memory allocated: %d bytes (%.2f kB)", totalBytesAlloced, float64(totalBytesAlloced) / 1024)

				if allocsPerOp > 0 {
					subB.Logf("%s expected 0 allocs/op, got %d\n%s", bm.name, allocsPerOp, remediationAction)
				}
				if bytesPerOp > 0 {
					subB.Logf("%s expected 0 bytes/op, got %d\n%s", bm.name, bytesPerOp, remediationAction)
				}
			})
		})
	}
}
