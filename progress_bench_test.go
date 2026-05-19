package progress

import (
	"io"
	"testing"
	"time"
)

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
//   go tool pprof -alloc_objects mem.pprof
//
//   (pprof) list progress
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

func BenchmarkRenderLoop(b *testing.B) {
	// set storeLastFrameHook (indirectly used by unit tests) to a no-op function since it allocates onto the heap
	storeLastFrameHook = func(*Progress, *[]byte) {}

	benchmarks        := []struct {
		name    string
		tracker statusTracker
	}{
		{ name: "Standard", tracker: getTracker(Standard, 1e12) },
		{ name: "Unique",   tracker: getTracker(Unique,   1e12) },
		{ name: "Fraction", tracker: getTracker(Fraction, 1e6 ) },
		{ name: "Percent",  tracker: getTracker(Percent,  0   ) },
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()

			tickTrigger := make(chan time.Time, 1)
			notify      := make(chan struct{},  1) // awaits the completion of a draw cycle; buffered to prevent deadlocks

			p := &Progress{
				tracker:    bm.tracker,
				output:     io.Discard,
				isTerminal: isTerminal,
				clock:      fakeClock{ c: tickTrigger },
				drawNotify: notify,
				stopChan:   make(chan struct{}),
				doneChan:   make(chan struct{}),
			}

			go p.renderLoop(b.Context())
			b.Cleanup(func() { p.Close() })

			// `for range b.N` is used instead of `for b.Loop()` because b.ResetTimer()
			// nullifies any initialization / startup allocations that would otherwise be
			// profiled, isolating the memory footprint of p.Report() and downstream calls

			b.ResetTimer() // ignore initialization / startup allocations

			taskCompleteMsg := "completed a task" // pre-allocated variable to eliminate interface / string literal heap escapes

			for range b.N {
				p.Report(10, taskCompleteMsg)
				tickTrigger <- time.Now()
				<-notify // draw() cycle completed
			}

			b.StopTimer()

			testP := &Progress{ // test p.Report() and downstream calls in isolation to verify 0-alloc behavior
				tracker:    bm.tracker,
				output:     io.Discard,
				clock:      fakeClock{ c: tickTrigger },
				drawNotify: notify,
				stopChan:   make(chan struct{}),
				doneChan:   make(chan struct{}),
			}

			allocs := testing.AllocsPerRun(100, func() { testP.Report(10, taskCompleteMsg) }) // sample 100 runs
			if allocs > 0 {
				remediationAction := "run `go test -run='^$' -memprofile=mem.pprof -bench=BenchmarkRenderLoop` to isolate the memory leak"
				b.Errorf("%s leaked memory: expected 0 allocs per run, got %.2f\n%s", bm.name, allocs, remediationAction)
			}
		})
	}
}
