package progress

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var opts = cmp.Options{
	cmp.AllowUnexported(Progress{}, standardTracker{}, uniqueTracker{}, percentTracker{}, fractionTracker{}, realClock{}),
	cmp.Transformer("unwrapBool",   func(t *atomic.Bool  ) bool   { return t.Load() }),
	cmp.Transformer("unwrapUint64", func(i *atomic.Uint64) uint64 { return i.Load() }),
	cmpopts.EquateComparable(atomic.Uint32{}, atomic.Uint64{}, atomic.Value{}, atomic.Pointer[string]{}),
	cmpopts.IgnoreFields(Progress{}, "output", "stopChan", "doneChan", "resizeChan", "closeOnce", "drawNotify"), // non-trivial to compare
}

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		totalUnits uint64
		wantTotal  uint64
	}{
		{
			name:       "weight-based accumulation",
			totalUnits: 100,
			wantTotal:  100,
		},
		{
			name:       "fractional path allocation",
			totalUnits: 0,
			wantTotal:  0,
		},
		{
			name:       "verify overflow safety",
			totalUnits: scale + 1000,
			wantTotal:  scale,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			buf := new(bytes.Buffer)
			got := New(ctx, tt.totalUnits, buf)
			t.Cleanup(func() { got.Close(ctx) })
			if diff := cmp.Diff(tt.wantTotal, got.total.Load(), opts...); diff != "" {
				t.Errorf("New(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if got.stopChan == nil || got.doneChan == nil || got.resizeChan == nil {
				t.Errorf("one or more channels were not initialized")
			}
		})
	}
}

func TestGetTermWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		output io.Writer
		want   uint16
	}{
		{
			name: "output file omitted",
			want: minWidth,
		},
		{
			name:   "output to os.Stderr",
			output: os.Stderr,
			want:   minWidth,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got uint16
			if tt.output == nil {
				got = getTermWidth()
			} else {
				f, _ := tt.output.(*os.File)
				got = getTermWidth(f)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("getTermWidth(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestInitialBudget(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want float64
	}{
		{
			name: "succeeds",
			want: float64(scale),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			p   := New(ctx, 0, io.Discard)
			t.Cleanup(func() { p.Close(ctx) })
			got := p.InitialBudget()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("InitialBudget(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestAddTotal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		units uint64
		want  uint64
	}{
		{
			name:  "less than scale added",
			units: 100,
			want:  100,
		},
		{
			name:  "more than scale added; total capped at scale",
			units: scale + 1000,
			want:  scale,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			p   := New(ctx, 0, io.Discard)
			p.AddTotal(tt.units)
			t.Cleanup(func() { p.Close(ctx) })
			if diff := cmp.Diff(tt.want, p.total.Load()); diff != "" {
				t.Errorf("AddTotal(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestReport(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		total     uint64
		unitsDone float64
		status    string
		want      uint64
	}{
		{
			name:      "weight-based; standard increments",
			total:     100,
			unitsDone: 1,
			status:    "completed 1 unit of work",
			want:      (1 * scale / 100) * 3,
		},
		{
			name:      "weight-based; high-precision boundary check",
			total:     scale,
			unitsDone: 1,
			status:    "completed 1 unit of work",
			want:      (1 * scale / scale) * 3,
		},
		{
			name:      "weight-based; reported work done cannot exceed total work",
			total:     100,
			unitsDone: 150,   // report 150% of total work done
			status:    "completed more work than budgeted",
			want:      scale, // budget not exceeded
		},
		{
			name:      "fractional path allocation; direct accumulation",
			total:     0,
			unitsDone: float64(scale) / 10,
			status:    "1/10 of the total work done",
			want:      (scale / 10) * 3,
		},
		{
			name:      "fractional path allocation; verify safe accumulation of a very large amount of work",
			total:     0,
			unitsDone: math.MaxUint64 / 4, // ~4612 times larger than scale (1e15)
			status:    "very large amount of work done",
			want:      scale,              // budget not exceeded
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			p   := New(ctx, tt.total, io.Discard)
			t.Cleanup(func() { p.Close(ctx) })
			for range 3 { p.Report(tt.unitsDone, tt.status) }
			if diff := cmp.Diff(tt.want, p.current.Load()); diff != "" {
				t.Errorf("current progress was not updated (-want +got):\n%s", diff)
			}
		})
	}
}

func pack(termWidth, pctSigDigits uint16) uint32 {
	return uint32(termWidth) << 16 | uint32(pctSigDigits)
}

func TestDraw(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		state      uint32
		statusText *string
		want       string
	}{
		{
			name:       "nominal terminal width of 80", // minWidth == 80
			state:      pack(80, 4700),                 // 80 - len("processing (100%): ") == 61
			statusText: new("just a small fish in a big sea"),
			want:       "processing ( 47%): just a small fish in a big sea",
		},
		{
			name:       "status message truncated from the left and prepended with an ellipsis",
			state:      pack(40, 7100), // 40 - len("processing (100%): ") == 21
			statusText: new("this is a very long status message that must be truncated"),
			want:       "processing ( 71%): ... must be truncated",
		},
		{
			name:       "status message truncated from the left with no ellipsis prepended (terminal too narrow)",
			state:      pack(22, 9300), // 22 - len("processing (100%): ") == 3
			statusText: new("short message"),
			want:       "processing ( 93%): age",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := new(bytes.Buffer)
			p   := &Progress{
				tracker:     &standardTracker{},
				output:      got,
				suffix:      defaultSuffix,
				staticWidth: len(prefix) + pctFieldLen + len(defaultSuffix),
			}

			p.draw(tt.state, tt.statusText)

			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("draw(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestRenderLoop(t *testing.T) {
	// TODO(jeff): improve this test
	t.Parallel()

	got         := new(bytes.Buffer)
	tickTrigger := make(chan time.Time, 1)
	notify      := make(chan struct{}, 1) // awaits the completion of a draw cycle, buffered to prevent deadlocks

	p := &Progress{
		tracker:     &standardTracker{},
		output:      got,
		clock:       &fakeClock{ c: tickTrigger },
		drawNotify:  notify,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
		staticWidth: len(prefix) + pctFieldLen + len(defaultSuffix),
	}

	tickAndExpectDraw := func() {
		tickTrigger <- time.Now()
		<-notify // draw() cycle completed
	}

	tickAndExpectSkip := func() {
		tickTrigger <- time.Now()
		for p.lastState.Load() != p.state.Load() { // wait until renderLoop has processed the state via the stage 2 check
			runtime.Gosched()
		}
		select {       // verify that the notify channel is empty
		case <-notify: // unexpected draw() cycle completed
			t.Errorf("redundant draw rendered")
		default:
		}
	}

	p.total.Store(100)
	p.state.Store(pack(minWidth, 0))

	ctx := t.Context()
	go p.renderLoop(ctx)

	p.Report(10, "working...")
	tickAndExpectDraw()

	p.Report(0, "working...") // redundant report
	tickAndExpectSkip()

	t.Cleanup(func() { p.Close(ctx) })
}

func TestFractionTrackerRedraw(t *testing.T) {
	t.Parallel()

	got         := new(bytes.Buffer)
	tickTrigger := make(chan time.Time, 1)
	notify      := make(chan struct{}, 1) // awaits the completion of a draw cycle, buffered to prevent deadlocks

	p := &Progress{
		tracker:     &fractionTracker{ total: "73" },
		output:      got,
		clock:       &fakeClock{ c: tickTrigger },
		drawNotify:  notify,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
		suffix:      defaultSuffix,
		staticWidth: len(prefix) + pctFieldLen + len(defaultSuffix),
	}

	p.total.Store(73)
	p.state.Store(pack(minWidth, 0))

	go p.renderLoop(t.Context())
	defer p.Close(t.Context())

	p.Report(11, "completed 11 units of work") // first report: 11/73
	tickTrigger <- time.Now()
	<-notify

	want := "processing ( 15%): 11/73"
	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("renderLoop() mismatch (-want +got):\n%s", diff)
	}

	got.Reset()

	p.Report(34, "completed another 34 units of work") // second report: 19/73
	tickTrigger <- time.Now()
	<-notify

	want = "processing ( 62%): 45/73"
	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("renderLoop() mismatch (-want +got):\n%s", diff)
	}
}

func TestClose(t *testing.T) {
	t.Parallel()
	staticWidth := len(prefix) + pctFieldLen + len(defaultSuffix)
	tests  := []struct {
		name     string
		total    uint64
		err      error
		wantOut  string
		wantProg *Progress
	}{
		{
			name:    "succeeds",
			total:   100,
			wantOut: "processing (100%): done\n",
			wantProg: &Progress{
				tracker:     &standardTracker{},
				buf:         []byte(""),
				clock:       &realClock{ dur: 16 * time.Millisecond },
				doneSeq:     "\n",
				lineTerm:    "\n",
				suffix:      defaultSuffix,
				staticWidth: staticWidth,
			},
		},
		{
			name:     "progress tracking was aborted",
			total:    200,
			err:      errors.New("aborted for some reason"),
			wantOut:  "stopped (aborted for some reason)\n",
			wantProg: &Progress{
				tracker:     &standardTracker{},
				buf:         []byte(""),
				clock:       &realClock{ dur: 16 * time.Millisecond },
				doneSeq:     "\n",
				lineTerm:    "\n",
				suffix:      defaultSuffix,
				staticWidth: staticWidth,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantProg.total.Store(tt.total)
			tt.wantProg.state.Store(pack(minWidth, 0))
			ctx, cancel := context.WithCancelCause(t.Context())
			got         := new(bytes.Buffer)
			p           := New(ctx, tt.total, got)
			cancel(tt.err)
			p.Close(ctx)
			if diff := cmp.Diff(tt.wantOut, got.String()); diff != "" {
				t.Errorf("Close(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantProg, p, opts...); diff != "" {
				t.Errorf("Close(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
