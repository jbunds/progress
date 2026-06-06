package progress

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestDraw(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		state      uint32
		statusText string
		want       string
	}{
		{
			name:       "nominal terminal width of 80", // minWidth == 80
			state:      pack(t, 80, 0.47),              // 80 - len("processing (100%): ") == 61
			statusText: "just a small fish in a big sea",
			want:       "processing ( 47%): just a small fish in a big sea\n",
		},
		{
			name:       "status message truncated from the left and prepended with an ellipsis",
			state:      pack(t, 40, 0.71), // 40 - len("processing (100%): ") == 21
			statusText: "this is a very long status message that must be truncated",
			want:       "processing ( 71%): …at must be truncated\n",
		},
		{
			name:       "status message truncated from the left with no ellipsis prepended (terminal too narrow)",
			state:      pack(t, 22, 0.93), // 22 - len("processing (100%): ") == 3
			statusText: "short message",
			want:       "processing ( 93%): …ge\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Progress{
				tracker:    getTracker(Standard, 0),
				output:     io.Discard,
				isTerminal: isTerminal,
			}
			p.tracker.store(0, tt.statusText)
			p.prepareTerminal()

			buf := make([]byte, 0, p.layout.bufCap(int(p.state.Load() >> 16)))
			p.draw(buf, tt.state)

			if diff := cmp.Diff(tt.want, p.lastFrameRendered()); diff != "" {
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
			state: pack(t, termWidth, 0.0094),
			want:  "processing (0.9%)\n",
		},
		{
			name:  "1.0%",
			state: pack(t, termWidth, 0.0095),
			want:  "processing (1.0%)\n",
		},
		{
			name:  "9.9%",
			state: pack(t, termWidth, 0.0994),
			want:  "processing (9.9%)\n",
		},
		{
			name:  "10%",
			state: pack(t, termWidth, 0.0995),
			want:  "processing ( 10%)\n",
		},
		{
			name:  "99%",
			state: pack(t, termWidth, 0.9949),
			want:  "processing ( 99%)\n",
		},
		{
			name:  "100%",
			state: pack(t, termWidth, 0.9950),
			want:  "processing (100%)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Progress{
				tracker:    getTracker(Percent, 0),
				output:     io.Discard,
				isTerminal: isTerminal,
			}
			p.prepareTerminal()

			buf := make([]byte, 0, p.layout.bufCap(int(p.state.Load() >> 16)))
			p.draw(buf, tt.state)

			if diff := cmp.Diff(tt.want, p.lastFrameRendered()); diff != "" {
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
			state:      pack(t, minWidth, 0.37),
			statusText: "working...",
			want:       "processing ( 37%): working...\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Progress{
				tracker:    getTracker(Unique, 0),
				output:     io.Discard,
				isTerminal: isTerminal,
			}
			p.tracker.store(0, tt.statusText)
			p.prepareTerminal()

			buf := make([]byte, 0, p.layout.bufCap(int(p.state.Load() >> 16)))
			p.draw(buf, tt.state)

			if diff := cmp.Diff(tt.want, p.lastFrameRendered()); diff != "" {
				t.Errorf("draw(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestFractionTrackerRedraw(t *testing.T) {
	t.Parallel()

	got         := new(bytes.Buffer)
	tickTrigger := make(chan time.Time, 1)
	notify      := make(chan struct{},  1) // awaits the completion of a draw cycle, buffered to prevent deadlocks

	p := &Progress{
		tracker:    getTracker(Fraction, 73),
		output:     got,
		isTerminal: isTerminal,
		clock:      fakeClock{ c: tickTrigger },
		drawNotify: notify,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
	}
	p.initBufPool()
	p.prepareTerminal()

	p.total.Store(73)
	p.state.Store(pack(t, minWidth, 0))

	go p.renderLoop(t.Context())
	t.Cleanup(func() { p.Close() })

	p.Report(11, "completed 11 units of work") // first report: 11/73
	tickTrigger <-time.Time{}
	<-notify

	tickTrigger <-time.Time{} // should skip redundant redraw
	<-notify

	wantFrame := "processing ( 15%): 11/73\n"
	want      := wantFrame
	if diff := cmp.Diff(wantFrame, p.lastFrameRendered()); diff != "" {
		t.Errorf("renderLoop() mismatch (-want +got):\n%s", diff)
	}

	p.Report(34, "completed another 34 units of work") // second report: 45/73

	wantFrame = "processing ( 62%): 45/73\n"
	want     += wantFrame

	for range 10 { // accommodate scheduler jitter and frame queuing by consuming notifications until we reach the expected state, otherwise fail fast
		tickTrigger <-time.Time{}
		<-notify
		if p.lastFrameRendered() == wantFrame { break }
	}

	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("renderLoop() mismatch (-want +got):\n%s", diff)
	}
}

func TestWriteString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		cols          int
		denom         int
		visCols       int
		isColored     bool
		str           string
		want          string
		wantIsColored bool
	}{
		{
			name:          "wide load",
			cols:          30,
			denom:         30 - 1,
			visCols:       20,
			str:           "🙃",
			want:          "\033[38;2;1;1;1;48;2;255;94;47m🙃",
			wantIsColored: true,
		},
		{
			name:      "reset",
			cols:      30,
			denom:     30 - 1,
			visCols:   30,
			isColored: true,
			str:       "foo",
			want:      "\033[0mfoo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := make([]byte, 0)
			ws  := &writeState{
				theme:      newThemeRegistry().get("sunset"),
				cols:       tt.cols,
				visCols:    tt.visCols,
				denom:      tt.denom,
				termWidth:  tt.cols,
				isTerminal: true,
				isColored:  tt.isColored,
			}
			buf = ws.writeString(buf, tt.str)
			if diff := cmp.Diff(tt.want, string(buf)); diff != "" {
				t.Errorf("writeString(%q) mismatch (-want +got):\n%s", tt.str, diff)
			}
			if diff := cmp.Diff(tt.wantIsColored, ws.isColored); diff != "" {
				t.Errorf("writeString(%q) mismatch (-want +got):\n%s", tt.str, diff)
			}
		})
	}
}

func TestWriteStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		isTerminal   bool
		pctSigDigits uint32
		status       string
		want         string
		wantErr      bool
	}{
		{
			name:         "output not a terminal",
			isTerminal:   false,
			pctSigDigits: 5377,
			status:       "shouting into the void",
			want:         "processing ( 54%): shouting into the void\n",
		},
		{
			name:         "output is a terminal",
			isTerminal:   true,
			pctSigDigits: 7731,
			status:       "working...",
			want:         ansiClearSeq +
			              "\033[38;2;255;255;255;48;2;48;25;52mp"  + "\033[38;2;255;255;255;48;2;54;24;52mr"  +
			              "\033[38;2;255;255;255;48;2;59;23;52mo"  + "\033[38;2;255;255;255;48;2;65;22;53mc"  +
			              "\033[38;2;255;255;255;48;2;71;21;53me"  + "\033[38;2;255;255;255;48;2;77;20;53ms"  +
			              "\033[38;2;255;255;255;48;2;82;19;53ms"  + "\033[38;2;255;255;255;48;2;88;18;53mi"  +
			              "\033[38;2;255;255;255;48;2;94;17;54mn"  + "\033[38;2;255;255;255;48;2;100;16;54mg" +
			              "\033[38;2;255;255;255;48;2;105;16;54m " + "\033[38;2;255;255;255;48;2;111;15;54m(" +
			              "\033[38;2;255;255;255;48;2;117;14;54m " + "\033[38;2;255;255;255;48;2;123;13;54m7" +
			              "\033[38;2;255;255;255;48;2;128;12;55m7" + "\033[38;2;255;255;255;48;2;134;11;55m%" +
			              "\033[38;2;255;255;255;48;2;140;10;55m)" + "\033[38;2;255;255;255;48;2;145;9;55m:"  +
			              "\033[38;2;255;255;255;48;2;151;8;55m "  + "\033[38;2;255;255;255;48;2;157;7;56mw"  +
			              "\033[38;2;255;255;255;48;2;163;6;56mo"  + "\033[38;2;255;255;255;48;2;168;5;56mr"  +
			              "\033[38;2;255;255;255;48;2;174;4;56mk"  + "\033[38;2;255;255;255;48;2;180;3;56mi"  +
			              "\033[38;2;255;255;255;48;2;186;2;57mn"  + "\033[38;2;255;255;255;48;2;191;1;57mg"  +
			              "\033[38;2;255;255;255;48;2;197;0;57m."  + "\033[38;2;255;255;255;48;2;200;2;57m."  +
			              "\033[38;2;255;255;255;48;2;203;6;57m."  +
			              "\033[30;48;2;205;9;56m "   + "\033[30;48;2;207;12;56m "  + "\033[30;48;2;209;15;56m "  +
			              "\033[30;48;2;211;19;56m "  + "\033[30;48;2;213;22;55m "  + "\033[30;48;2;215;25;55m "  +
			              "\033[30;48;2;217;29;55m "  + "\033[30;48;2;220;32;55m "  + "\033[30;48;2;222;35;55m "  +
			              "\033[30;48;2;224;39;54m "  + "\033[30;48;2;226;42;54m "  + "\033[30;48;2;228;45;54m "  +
			              "\033[30;48;2;230;48;54m "  + "\033[30;48;2;232;52;53m "  + "\033[30;48;2;234;55;53m "  +
			              "\033[30;48;2;237;58;53m "  + "\033[30;48;2;239;62;53m "  + "\033[30;48;2;241;65;53m "  +
			              "\033[30;48;2;243;68;52m "  + "\033[30;48;2;245;72;52m "  + "\033[30;48;2;247;75;52m "  +
			              "\033[30;48;2;249;78;52m "  + "\033[30;48;2;251;81;51m "  + "\033[30;48;2;254;85;51m "  +
			              "\033[30;48;2;255;88;50m "  + "\033[30;48;2;255;92;48m "  + "\033[30;48;2;255;97;46m "  +
			              "\033[30;48;2;255;101;45m " + "\033[30;48;2;255;105;43m " + "\033[30;48;2;255;109;41m " +
			              "\033[30;48;2;255;113;39m " + "\033[30;48;2;255;117;37m " + ansiResetAttr +
			              ansiLineTerminator,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Progress{
				tracker:    getTracker(Standard, 3),
				output:     io.Discard,
				isTerminal: func(any) bool { return tt.isTerminal },
				theme:      newThemeRegistry().get("sunset"),
			}
			p.prepareTerminal()
			p.state.Store(pack(t, 80, 0))

			buf := make([]byte, 0, p.layout.bufCap(int(p.state.Load() >> 16)))
			_, _ = p.writeStatus(buf, tt.pctSigDigits, tt.status, false)

			if diff := cmp.Diff(tt.want, p.lastFrameRendered()); diff != "" {
				t.Errorf("writeStatus(%d, %q, %t) mismatch (-want +got):\n%s", tt.pctSigDigits, tt.status, false, diff)
			}
		})
	}
}
