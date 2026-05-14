package progress

import (
	"bytes"
	"testing"
	"time"
	"unique"

	"github.com/google/go-cmp/cmp"
)

func buf() *[]byte {
	buf := make([]byte, 0, 128)
	return &buf
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
			state:      pack(80, 0.47),                 // 80 - len("processing (100%): ") == 61
			statusText: new("just a small fish in a big sea"),
			want:       "processing ( 47%): just a small fish in a big sea\n",
		},
		{
			name:       "status message truncated from the left and prepended with an ellipsis",
			state:      pack(40, 0.71), // 40 - len("processing (100%): ") == 21
			statusText: new("this is a very long status message that must be truncated"),
			want:       "processing ( 71%): …at must be truncated\n",
		},
		{
			name:       "status message truncated from the left with no ellipsis prepended (terminal too narrow)",
			state:      pack(22, 0.93), // 22 - len("processing (100%): ") == 3
			statusText: new("short message"),
			want:       "processing ( 93%): …ge\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := new(bytes.Buffer)
			p   := &Progress{
				tracker: getTracker(Standard, 0),
				output:  got,
			}
			p.buf.Store(buf())

			p.draw(tt.state, tt.statusText)

			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("draw(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestPercentTrackerDraw(t *testing.T) {
	t.Parallel()
	suffix    := "%)"
	termWidth := len(prefix) + pctFieldLen + len(suffix)
	tests     := []struct {
		name  string
		state uint32
		want  string
	}{
		{
			name:  "0.9%",
			state: pack(termWidth, 0.0094),
			want:  "processing (0.9%)\n",
		},
		{
			name:  "1.0%",
			state: pack(termWidth, 0.0095),
			want:  "processing (1.0%)\n",
		},
		{
			name:  "9.9%",
			state: pack(termWidth, 0.0994),
			want:  "processing (9.9%)\n",
		},
		{
			name:  "10%",
			state: pack(termWidth, 0.0995),
			want:  "processing ( 10%)\n",
		},
		{
			name:  "99%",
			state: pack(termWidth, 0.9949),
			want:  "processing ( 99%)\n",
		},
		{
			name:  "100%",
			state: pack(termWidth, 0.9950),
			want:  "processing (100%)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := new(bytes.Buffer)
			p   := &Progress{
				tracker: getTracker(Percent, 0),
				output:  got,
			}
			p.buf.Store(buf())

			p.draw(tt.state, "")

			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("draw(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestUniqueTrackerDraw(t *testing.T) {
	t.Parallel()
	tests  := []struct {
		name       string
		total      uint64
		state      uint32
		statusText string
		want       string
	}{
		{
			name:       "succeeds",
			total:      100,
			state:      pack(minWidth, 0.37),
			statusText: "working...",
			want:       "processing ( 37%): working...\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := new(bytes.Buffer)
			p   := &Progress{
				tracker: getTracker(Unique, 0),
				output:  got,
			}
			p.buf.Store(buf())

			p.draw(tt.state, unique.Make(tt.statusText))

			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("draw(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestFractionTrackerRedraw(t *testing.T) {
	t.Parallel()

	got         := new(bytes.Buffer)
	tickTrigger := make(chan time.Time, 1)
	notify      := make(chan struct{}, 1) // awaits the completion of a draw cycle, buffered to prevent deadlocks

	p := &Progress{
		tracker:    getTracker(Fraction, 73),
		output:     got,
		clock:      &fakeClock{ c: tickTrigger },
		drawNotify: notify,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
	}

	p.buf.Store(buf())
	p.total.Store(73)
	p.state.Store(pack(minWidth, 0))

	go p.renderLoop(t.Context())
	t.Cleanup(func() { p.Close() })

	p.Report(11, "completed 11 units of work") // first report: 11/73
	tickTrigger <- time.Now()
	<-notify

	want := "processing ( 15%): 11/73\n"
	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("renderLoop() mismatch (-want +got):\n%s", diff)
	}

	got.Reset()

	p.Report(34, "completed another 34 units of work") // second report: 45/73
	tickTrigger <- time.Now()
	<-notify

	want = "processing ( 62%): 45/73\n"
	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("renderLoop() mismatch (-want +got):\n%s", diff)
	}
}
