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
		cmpopts.IgnoreFields(Progress{}, "stopChan", "doneChan", "input", "output"), // non-trivial to compare
	}
	tests := []struct {
		name  string
		total int
		want  *Progress
	}{
		{
			name:   "succeeds",
			total:  int(100),
			want:   &Progress{
				current: int64(0),
				total:   int64(100),
				clock:   &realClock{ d: 16 * time.Millisecond },
				fd:      2,
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

func TestUpdate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want int64
	}{
		{
			name: "3 updates performed",
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Progress{
				total:  int64(100),
				output: io.Discard,
			}
			for range 3 { p.Update("updating") }
			if diff := cmp.Diff(tt.want, p.current); diff != "" {
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

	p := &Progress{
		total:      100,
		current:    0,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
		output:     &buf,
		clock:      &fakeClock{c: tickTrigger},
		drawNotify: notify,
	}

	go p.renderLoop()

	p.input.Store("starting...")
	tickTrigger <- time.Now()
	<-notify

	atomic.StoreInt64(&p.current, 50)
	p.input.Store("50% complete...")
	tickTrigger <- time.Now()
	<-notify

	atomic.StoreInt64(&p.current, 100)
	p.input.Store("done")
	close(p.stopChan)
	<-notify
	<-p.doneChan

	want := "\r\033[2Kprocessing (  0%): starting..."     +
	        "\r\033[2Kprocessing ( 50%): 50% complete..." +
	        "\r\033[2Kprocessing (100%): done"            +
	        "\033[2K\r\033[?25h" // cursor restoration ANSI escape sequence

	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Errorf("renderLoop mismatch (-want +got):\n%s", diff)
	}
}
