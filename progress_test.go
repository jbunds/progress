package progress

import (
	"bytes"
	"io"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NewProgress(tt.total, io.Discard)
			tt.want.input.Store("")
			defer got.Close()
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
			defer p.Close()
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
		name string
		want uint64
	}{
		{
			name: "3 updates performed",
			want: (scale / 100) * 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := NewProgress(100, io.Discard)
			defer p.Close()
			for range 3 { p.Report(1, "updating") }
			if diff := cmp.Diff(tt.want, p.current.Load()); diff != "" {
				t.Errorf("current progress was not updated (-want +got):\n%s", diff)
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
