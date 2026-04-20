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
	cmpopts.IgnoreFields(Progress{}, "stopChan", "doneChan", "output", "closeOnce"), // non-trivial to compare
}

func TestNewProgress(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		total  uint64
		want   *Progress
	}{
		{
			name:   "weight-based accumulation",
			total:  uint64(100),
			want:   &Progress{
				total: uint64(100),
				clock: &realClock{ dur: 16 * time.Millisecond },
				width: 80,
			},
		},
		{
			name:   "fractional path allocation",
			total:  uint64(0),
			want:   &Progress{
				total: uint64(0),
				clock: &realClock{ dur: 16 * time.Millisecond },
				width: 80,
			},
		},
		{
			name:  "verify overflow safety",
			total: maxSafeUnits + 1000,
			want:  &Progress{
				total: maxSafeUnits,
				clock: &realClock{ dur: 16 * time.Millisecond },
				width: 80,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NewProgress(tt.total, io.Discard)
			t.Cleanup(got.Close)
			tt.want.input.Store("")
			if diff := cmp.Diff(tt.want, got, opts...); diff != "" {
				t.Errorf("NewProgress(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if got.stopChan == nil { t.Errorf("stopChan was not initialized") }
			if got.doneChan == nil { t.Errorf("doneChan was not initialized") }
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
		want uint64
	}{
		{
			name: "succeeds",
			want: scale,
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
		unitsDone uint64
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
			unitsDone: 150,       // 150% of 100% of work reported done
			status:    "completed more work than budgeted",
			want:      scale * 3, // each report adds 100% of scale
		},
		{
			name:      "fractional path allocation; direct accumulation",
			total:     0,
			unitsDone: scale / 10,
			status:    "1/10 of the total work done",
			want:      (scale / 10) * 3,
		},
		{
			name:      "fractional path allocation; verify accumulation safety",
			total:     0,
			unitsDone: math.MaxUint64 / 4,
			status:    "large amount of work done",
			want:      (math.MaxUint64 / 4) * 3,
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
	ansiEscSeq    := "\r\033[2K"
	ansiEscSeqLen := len(ansiEscSeq)
	tests := []struct {
		name    string
		width   int
		current uint64
		status  string
		want    string
		wantLen int // the expected length of the message printed to the terminal
	}{
		{
			name:    "standard width",
			width:   80,
			current: scale / 2,
			status:  "working...",
			want:    ansiEscSeq    +     "processing ( 50%): "  +     "working...",
			wantLen: ansiEscSeqLen + len("processing ( 50%): ") + len("working..."),
		},
		{
			name:    "narrow terminal; status truncated",
			width:   30,
			current: 0,
			status:  "this status message is much too long to fit within the width of the terminal",
			want:    ansiEscSeq    +     "processing (  0%): "  +     "...terminal",
			wantLen: ansiEscSeqLen + len("processing (  0%): ") + len("...terminal"),
		},
		{
			name:    "very narrow terminal; status omitted",
			width:   10,
			current: 0,
			status:  "no room for status",
			want:    ansiEscSeq    +     "processing (  0%): ",
			wantLen: ansiEscSeqLen + len("processing (  0%): "),
		},
		{
			name:    "verify overflow protection",
			width:   80,
			current: math.MaxUint64,
			status:  "massive amout of work",
			want:    ansiEscSeq    +     "processing (100%): "  +     "done",
			wantLen: ansiEscSeqLen + len("processing (100%): ") + len("done"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := new(bytes.Buffer)
			p := &Progress{
				total:  100,
				width:  tt.width,
				output: got,
			}
			
			p.current.Store(tt.current)
			p.input.Store(tt.status)
			
			p.draw()

			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("draw(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantLen, len(got.String())); diff != "" {
				t.Errorf("draw(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestRenderLoop(t *testing.T) {
	t.Parallel()

	got := new(bytes.Buffer)
	tickTrigger := make(chan time.Time)
	notify      := make(chan struct{}) // sync channel

	tick := func() {
		tickTrigger <- time.Now()
		<-notify
	}

	p := &Progress{
		total:      100,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
		output:     got,
		width:      80,
		clock:      &fakeClock{ chn: tickTrigger },
		drawNotify: notify,
	}

	go p.renderLoop()

	p.input.Store("starting...")
	tick()

	p.Report(40, "40% complete...")
	tick()

	p.Report(60, "done")
	tick()

	close(p.stopChan)
	<-notify
	<-p.doneChan

	want := "\r\033[2Kprocessing (  0%): starting..."     + // tick 1
	        "\r\033[2Kprocessing ( 40%): 40% complete..." + // tick 2 (Report(40, ...))
	        "\r\033[2Kprocessing (100%): done"            + // tick 3 (Report(60, ...))
	        "\033[2K\r\033[?25h"                            // cursor restoration

	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("renderLoop mismatch (-want +got):\n%s", diff)
	}
}

func TestClose(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		wantOut  string
		wantProg *Progress
	}{
		{
			name:     "succeeds",
			wantOut:  "\033[?25l"                        + // hide the cursor
			          "\r\033[2Kprocessing (100%): done" + // forcibly updated status
			          "\033[2K\r\033[?25h",                // cursor restored
			wantProg: &Progress{
				clock: &realClock{ dur: 16 * time.Millisecond },
				width: 80,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantProg.current.Store(scale)
			tt.wantProg.input.Store("done")
			tt.wantProg.drawnDone.Store(true)
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
