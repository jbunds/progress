package progress

import (
	"bytes"
	"io"
	"math"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var opts = cmp.Options{
	cmp.AllowUnexported(Progress{}, realClock{}),
	cmp.Transformer("unwrapAtomic", func(v atomic.Value) any { return v.Load() }),
	cmpopts.EquateComparable(atomic.Bool{}, atomic.Uint64{}),
	cmpopts.IgnoreFields(Progress{}, "mu", "stopChan", "doneChan", "resizeChan", "output", "closeOnce"), // non-trivial to compare
}

func TestNewProgress(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		totalUnits uint64
		wantTotal  uint64
		wantBuf    []byte
	}{
		{
			name:       "weight-based accumulation",
			totalUnits: 100,
			wantTotal:  100,
			wantBuf:    []byte("\033[?25l"), // hide the cursor
		},
		{
			name:       "fractional path allocation",
			totalUnits: 0,
			wantTotal:  0,
			wantBuf:    []byte("\033[?25l"), // hide the cursor
		},
		{
			name:       "verify overflow safety",
			totalUnits: maxSafeUnits + 1000,
			wantTotal:  maxSafeUnits,
			wantBuf:    []byte("\033[?25l"), // hide the cursor
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := new(bytes.Buffer)
			got := NewProgress(tt.totalUnits, buf)
			t.Cleanup(got.Close)
			if diff := cmp.Diff(tt.wantTotal, got.total.Load(), opts...); diff != "" {
				t.Errorf("NewProgress(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantBuf, got.buf); diff != "" {
				t.Errorf("NewProgress(%q) mismatch (-want +got):\n%s", tt.name, diff)
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
				got = getWidth()
			} else {
				f, _ := tt.output.(*os.File)
				got = getWidth(f)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("getWidth(%q) mismatch (-want +got):\n%s", tt.name, diff)
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
			p := NewProgress(0, io.Discard)
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
			p := NewProgress(tt.total, io.Discard)
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
	ansiEscSeq := "\r\033[2K\r"
	tests := []struct {
		name    string
		width   int
		current uint64
		status  string
		redraws int
		want    string
	}{
		{
			name:    "standard width",
			width:   80,
			current: scale / 2,
			status:  "working...",
			redraws: 1,
			want:    ansiEscSeq + "processing ( 50%): " + "working...",
		},
		{
			name:    "narrow terminal; status truncated",
			width:   30,
			current: 0,
			status:  "this status message is much too long to fit within the width of the terminal",
			redraws: 1,
			want:    ansiEscSeq + "processing (0.0%): " + "...terminal",
		},
		{
			name:    "very narrow terminal; status omitted",
			width:   10,
			current: 0,
			status:  "no room for status",
			redraws: 1,
			want:    ansiEscSeq + "processing (0.0%): ",
		},
		{
			name:    "skip redundant redraws",
			width:   80,
			current: 0,
			status:  "render this only once",
			redraws: 3,
			want:    ansiEscSeq + "processing (0.0%): " + "render this only once",
		},
		{
			name:    "verify final completion frame is rendered only once",
			width:   80,
			current: scale,
			status:  "done",
			redraws: 3,
			want:    "", // Close() handles the final completion frame when all work is done
		},
		{
			name:    "verify overflow protection",
			width:   80,
			current: math.MaxUint64,
			status:  "massive amout of work",
			redraws: 1,
			want:    "", // Close() handles the final completion frame when all work is done
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := new(bytes.Buffer)
			p := &Progress{
				width:  tt.width,
				output: got,
			}

			p.current.Store(tt.current)
			p.input.Store(tt.status)
			
			for i := 0; i < tt.redraws; i++ { // call p.draw() tt.redraws times
				p.draw()
			}

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
	notify      := make(chan struct{}) // sync channel

	tick := func() {
		tickTrigger <- time.Now()
		<-notify
	}

	p := &Progress{
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
		output:     got,
		width:      80,
		clock:      &fakeClock{ chn: tickTrigger },
		drawNotify: notify,
	}
	p.total.Store(100)
	p.input.Store("")

	go p.renderLoop()

	p.input.Store("starting...")
	tick()

	p.Report(40, "40% complete...")
	tick()

	p.Report(60, "done")
	tick()

	p.Close()

	want := "\r\033[2K\rprocessing (0.0%): starting..."     + // tick 1
	        "\r\033[2K\rprocessing ( 40%): 40% complete..." + // tick 2 (Report(40, ...))
	        "\r\033[2K\rprocessing (100%): done\r"          + // tick 3 (Report(60, ...))
	        "\033[?25h"                                       // cursor restoration

	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("renderLoop mismatch (-want +got):\n%s", diff)
	}
}

func TestClose(t *testing.T) {
	t.Parallel()

	output := "\r\033[2K\rprocessing (100%): done\r" + // final completion frame
	          "\033[?25h"                              // restore the cursor

	tests  := []struct {
		name     string
		wantOut  string
		wantProg *Progress
	}{
		{
			name:     "succeeds",
			wantOut:  "\033[?25l" + output,
			wantProg: &Progress{
				clock: &realClock{ dur: 16 * time.Millisecond },
				width: 80,
				buf:   []byte(output),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantProg.input.Store("")
			got := new(bytes.Buffer)
			p   := NewProgress(0, got)
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
