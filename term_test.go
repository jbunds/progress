package progress

import (
	"context"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestPrepareTerminal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		isTerminal         bool
		wantClearSeq       string
		wantDoneSeq        string
		wantLineTerminator string
	}{
		{
			name:               "is a terminal",
			isTerminal:         true,
			wantClearSeq:       "\r\033[K\033[?2026h\033[?7l",
			wantDoneSeq:        "\033[0m\r\033[?25h\033[?7h",
			wantLineTerminator: "\033[0m\033[?2026l",
		},
		{
			name:               "is not a terminal",
			isTerminal:         false,
			wantClearSeq:       "",
			wantDoneSeq:        "\n",
			wantLineTerminator: "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Progress{
				output:     os.Stderr,
				tracker:    getTracker(Standard, 0),
				isTerminal: func(any) bool { return tt.isTerminal },
			}
			p.prepareTerminal()
			got := p.layout
			if diff := cmp.Diff(tt.wantClearSeq, got.clearSeq); diff != "" {
				t.Errorf("prepareTerminal(%q) clearSeq mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantDoneSeq, got.doneSeq); diff != "" {
				t.Errorf("prepareTerminal(%q) doneSeq mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantLineTerminator, got.lineTerminator); diff != "" {
				t.Errorf("prepareTerminal(%q) lineTerminator mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestHandleResize(t *testing.T) {
	t.Parallel()

	fakeClock := fakeClock{ c: make(chan time.Time, 1) }
	notify    := make(chan struct{}, 1)

	mockTermWidth     := minWidth
	mockResizeHandler := func() int { return mockTermWidth }

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	p := New(ctx, 0, io.Discard,
		withClock(fakeClock),
		withResizeHandler(mockResizeHandler),
		WithIsTerminalFunc(func(any) bool { return true }))
	t.Cleanup(func() { p.Close() })
	p.drawNotify = notify // drawNotify signals the completion of a draw cycle

	p.resizeChan <-syscall.SIGWINCH

	<-notify // await a draw cycle to ensure the resize event has been processed

	zeroCapBuf     := make([]byte, 0)
	returnedBuf    := p.handleResize(zeroCapBuf)
	expectedMinCap := p.layout.bufCap(minWidth)
	if cap(returnedBuf) < expectedMinCap {
		t.Errorf("handleResize() mismatch; want >= %d, got %d", expectedMinCap, cap(returnedBuf))
	}

	select {
	case <-notify: // flush any synchronous notify token generated via the explicit p.handleResize call
	default:
	}

	mockTermWidth = 120
	p.resizeChan <- syscall.SIGWINCH
	<-notify // await a draw cycle to ensure the resize event has been processed

	want := uint32(120)
	got  := p.state.Load() >> 16

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("handleResize() mismatch (-want +got):\n%s", diff)
	}

	cancel()
	<-p.doneChan
}

func TestGetResizedTermWidth(t *testing.T) {
	t.Parallel()
	p := New(t.Context(), 0, io.Discard)
	t.Cleanup(func() { p.Close() })
	want := p.layout.staticWidth
	got  := p.getResizedTermWidth()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("getResizedTermWidth() mismatch (-want +got):\n%s", diff)
	}
}

func TestGetTermWidth(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil { t.Error(err) }
	t.Cleanup(func() { if err := r.Close(); err != nil { t.Log(err) } })
	t.Cleanup(func() { if err := w.Close(); err != nil { t.Log(err) } })

	tests := []struct {
		name   string
		output *os.File
		want   int
	}{
		{
			name:  "falls back to minWidth for non-terminal files",
			output: w, // pipes have no width
			want:   minWidth,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getTermWidth(tt.output)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("getTermWidth(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestGetFD(t *testing.T) {
	t.Parallel()
	w    := "not a file"
	got  := getFD(w)
	want := -1
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("getFD(%q) mismatch (-want +got):\n%s", w, diff)
	}
}
