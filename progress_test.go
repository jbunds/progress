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
	cmpopts.IgnoreUnexported(atomic.Bool{}, atomic.Uint64{}, atomic.Pointer[snapshot]{}),
	cmp.AllowUnexported(Progress{}, realClock{}, snapshot{}),
	cmp.Transformer("unwrapBool",     func(t *atomic.Bool             )  bool     { return t.Load() }),
	cmp.Transformer("unwrapUint64",   func(i *atomic.Uint64           )  uint64   { return i.Load() }),
	cmp.Transformer("unwrapSnapshot", func(p *atomic.Pointer[snapshot]) *snapshot { return p.Load() }),
	cmpopts.IgnoreFields(Progress{}, // the following are non-trivial to compare
		"output",
		"stopChan",
		"doneChan",
		"resizeChan",
		"closeOnce",
		"drawNotify",
	),
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
			totalUnits: maxSafeUnits + 1000,
			wantTotal:  maxSafeUnits,
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

func TestGetWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		output io.Writer
		want   uint16
	}{
		{
			name: "output file omitted",
			want: 80,
		},
		{
			name:   "output to os.Stderr",
			output: os.Stderr,
			want:   80,
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
			total:     maxSafeUnits,
			unitsDone: 1,
			status:    "completed 1 unit of work",
			want:      (1 * scale / maxSafeUnits) * 3,
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
			unitsDone: math.MaxUint64 / 4, // ~4.6 million times larger than scale
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

func TestDraw(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		snapshot *snapshot
		want     string
	}{
		{
			name:    "succeeds",
			snapshot: &snapshot{
				input:        "working...",
				pctSigDigits: uint16(5000),
				termWidth:    80,
			},
			want: "processing ( 50%): working...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := new(bytes.Buffer)
			p   := &Progress{ output: got }

			p.draw(tt.snapshot)

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
		output:      got,
		clock:       &fakeClock{ c: tickTrigger },
		drawNotify:  notify,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
		staticWidth: len(prefix) + pctFieldLen + len(suffix),
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
	p.state.Store(&snapshot{
		input:     "",
		termWidth: 80,
	})

	ctx := t.Context()
	go p.renderLoop(ctx, false)

	p.Report(10, "working...")
	tickAndExpectDraw()

	p.Report(0, "working...") // redundant report
	tickAndExpectSkip()

	t.Cleanup(func() { p.Close(ctx) })
}

func TestClose(t *testing.T) {
	t.Parallel()
	staticWidth := len(prefix) + pctFieldLen + len(suffix)
	tests  := []struct {
		name     string
		err      error
		wantOut  string
		wantProg *Progress
	}{
		{
			name:    "succeeds",
			wantOut: "processing (100%): done\n",
			wantProg: &Progress{
				buf:         []byte(""),
				clock:       &realClock{ dur: 16 * time.Millisecond },
				doneSeq:     "\n",
				lineTerm:    "\n",
				staticWidth: staticWidth,
			},
		},
		{
			name:     "progress tracking was aborted",
			err:      errors.New("aborted for some reason"),
			wantOut:  "stopped (aborted for some reason)\n",
			wantProg: &Progress{
				buf:         []byte(""),
				clock:       &realClock{ dur: 16 * time.Millisecond },
				doneSeq:     "\n",
				lineTerm:    "\n",
				staticWidth: staticWidth,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantProg.state.Store(&snapshot{
				input:     "",
				termWidth: 80,
			})
			ctx, cancel := context.WithCancelCause(t.Context())
			got := new(bytes.Buffer)
			p   := New(ctx, 100, got)
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
