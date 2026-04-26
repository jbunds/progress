package progress

import (
	"bytes"
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
			buf := new(bytes.Buffer)
			got := New(tt.totalUnits, buf)
			t.Cleanup(got.Close)
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
		want   int
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
			got := 0
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
			p := New(0, io.Discard)
			t.Cleanup(p.Close)
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
			p := New(tt.total, io.Discard)
			t.Cleanup(p.Close)
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
	t.Parallel()

	got         := new(bytes.Buffer)
	tickTrigger := make(chan time.Time)
	notify      := make(chan struct{}, 1) // sync channel, buffered to prevent deadlocks

	p := &Progress{
		output:      got,
		clock:       &fakeClock{ chn: tickTrigger },
		drawNotify:  notify,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
		staticWidth: len(prefix) + pctFieldLen + len(suffix),
	}

	tickAndExpectDraw := func() {
		tickTrigger <- time.Now()
		<-notify // wait for the draw signal
	}

	tickAndExpectSkip := func() {
		tickTrigger <- time.Now()
		for p.lastState.Load() != p.state.Load() { // wait until renderLoop has processed the state via the stage 2 check
			runtime.Gosched()
		}
		select { // verify notify is empty
		case <-notify:
			t.Errorf("redundant draw rendered")
		default:
		}
	}

	p.total.Store(100)
	p.state.Store(&snapshot{
		input:     "",
		termWidth: 80,
	})

	go p.renderLoop(false)

	p.Report(10, "working...")
	tickAndExpectDraw()

	p.Report(0, "working...") // redundant report
	tickAndExpectSkip()

	t.Cleanup(p.Close)
}

func TestClose(t *testing.T) {
	t.Parallel()
	tests  := []struct {
		name     string
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
				staticWidth: len(prefix) + pctFieldLen + len(suffix),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantProg.state.Store(&snapshot{
				input:     "",
				termWidth: 80,
			})
			got := new(bytes.Buffer)
			p   := New(0, got)
			p.Close()
			if diff := cmp.Diff(tt.wantOut, got.String()); diff != "" {
				t.Errorf("Close(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantProg, p, opts...); diff != "" {
				t.Errorf("Close(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
