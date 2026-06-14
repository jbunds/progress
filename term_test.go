package progress

import (
	"context"
	"io"
	"os"
	"testing"

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
			got := p.tracker.layout()
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

	mockTermWidth := minWidth

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	p := New(ctx, 0, io.Discard,
		withResizeHandler(func() int { return mockTermWidth }),
		WithIsTerminalFunc(func(any) bool { return true }))
	t.Cleanup(func() { p.Close() })

	wantTermWidth := uint32(mockTermWidth)
	gotTermWidth  := p.state.Load() >> 16

	if diff := cmp.Diff(wantTermWidth, gotTermWidth); diff != "" {
		t.Errorf("handleResize() mismatch (-want +got):\n%s", diff)
	}

	zeroCapBuf := make([]byte, 0)

	wantBufCap := p.tracker.layout().bufCap(mockTermWidth)
	gotBufCap  := cap(p.handleResize(zeroCapBuf))

	if diff := cmp.Diff(wantBufCap, gotBufCap); diff != "" {
		t.Errorf("handleResize() mismatch (-want +got):\n%s", diff)
	}

	mockTermWidth = 120

	wantBufCap    = p.tracker.layout().bufCap(mockTermWidth)
	gotBufCap     = cap(p.handleResize(zeroCapBuf))

	wantTermWidth = uint32(mockTermWidth)
	gotTermWidth  = p.state.Load() >> 16

	if diff := cmp.Diff(wantBufCap, gotBufCap); diff != "" {
		t.Errorf("handleResize() mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(wantTermWidth, gotTermWidth); diff != "" {
		t.Errorf("handleResize() mismatch (-want +got):\n%s", diff)
	}
}

func TestGetResizedTermWidth(t *testing.T) {
	t.Parallel()
	p := New(t.Context(), 0, io.Discard)
	t.Cleanup(func() { p.Close() })
	want := p.tracker.layout().staticWidth
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

	t.Run("falls back to minWidth for non-terminal files", func(t *testing.T) {
		t.Parallel()
		got := getTermWidth(w)
		if diff := cmp.Diff(minWidth, got); diff != "" {
			t.Errorf("getTermWidth() mismatch (-want +got):\n%s", diff)
		}
	})
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
