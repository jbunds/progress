package progress

import (
	"bytes"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNewProgress(t *testing.T) {
	t.Parallel()
	opts := []cmp.Option{
		cmp.AllowUnexported(Progress{}, realClock{}),
		cmpopts.EquateComparable(atomic.Uint64{}, atomic.Bool{}),
		cmpopts.IgnoreFields(Progress{}, "stopChan", "doneChan", "input", "output", "closeOnce"), // non-trivial to compare
	}
	tests := []struct {
		name  string
		total uint64
		want  *Progress
	}{
		{
			name:   "succeeds",
			total:  uint64(100),
			want:   &Progress{
				total: uint64(100),
				clock: &realClock{ d: 16 * time.Millisecond },
				fd:    2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NewProgress(tt.total, io.Discard)
			if diff := cmp.Diff(tt.want, got, opts...); diff != "" {
				t.Errorf("NewProgress(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if got.stopChan == nil {
				t.Errorf("stopChan was not initialized")
			}
			if got.doneChan == nil {
				t.Errorf("doneChan was not initialized")
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
			want: 30_000_000_000_000_000, // (3 units / 100 total) * 1e18 == 3e16
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := NewProgress(100, io.Discard)
			for range 3 { p.Report(1, "updating") }
			if diff := cmp.Diff(tt.want, p.current.Load()); diff != "" {
				t.Errorf("current progress was not updated (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRenderLoop(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
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
		output:     &buf,
		clock:      &fakeClock{c: tickTrigger},
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

	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Errorf("renderLoop mismatch (-want +got):\n%s", diff)
	}
}
