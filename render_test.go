package progress

import (
	"bytes"
	"io"
	"testing"
	"time"
	"unique"

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
				tracker:        getTracker(Standard, 0),
				output:         io.Discard,
				isTerminalFunc: isTerminal,
			}
			p.prepareTerminal()

			buf := make([]byte, 0, p.layout.bufCap(int(p.state.Load() >> 16)))
			p.draw(&buf, tt.state, tt.statusText)

			if diff := cmp.Diff(tt.want, p.lastRenderedFrame()); diff != "" {
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
				tracker:        getTracker(Percent, 0),
				output:         io.Discard,
				isTerminalFunc: isTerminal,
			}
			p.prepareTerminal()

			buf := make([]byte, 0, p.layout.bufCap(int(p.state.Load() >> 16)))
			p.draw(&buf, tt.state, "")

			if diff := cmp.Diff(tt.want, p.lastRenderedFrame()); diff != "" {
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
				tracker:        getTracker(Unique, 0),
				output:         io.Discard,
				isTerminalFunc: isTerminal,
			}
			p.prepareTerminal()

			buf := make([]byte, 0, p.layout.bufCap(int(p.state.Load() >> 16)))
			p.draw(&buf, tt.state, unique.Make(tt.statusText))

			if diff := cmp.Diff(tt.want, p.lastRenderedFrame()); diff != "" {
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
		tracker:        getTracker(Fraction, 73),
		output:         got,
		isTerminalFunc: isTerminal,
		clock:          &fakeClock{ c: tickTrigger },
		drawNotify:     notify,
		stopChan:       make(chan struct{}),
		doneChan:       make(chan struct{}),
	}
	p.prepareTerminal()

	p.total.Store(73)
	p.state.Store(pack(t, minWidth, 0))

	go p.renderLoop(t.Context())
	t.Cleanup(func() { p.Close() })

	p.Report(11, "completed 11 units of work") // first report: 11/73
	tickTrigger <-time.Now()
	<-notify

	tickTrigger <-time.Now() // should skip redundant redraw
	<-notify

	wantFrame := "processing ( 15%): 11/73\n"
	want      := wantFrame
	if diff := cmp.Diff(wantFrame, p.lastRenderedFrame()); diff != "" {
		t.Errorf("renderLoop() mismatch (-want +got):\n%s", diff)
	}

	p.Report(34, "completed another 34 units of work") // second report: 45/73

	wantFrame = "processing ( 62%): 45/73\n"
	want     += wantFrame

	for range 10 { // accommodate scheduler jitter and frame queuing by consuming notifications until we reach the expected state, otherwise fail fast
		tickTrigger <-time.Now()
		<-notify
		if p.lastRenderedFrame() == wantFrame { break }
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
			want:          "\033[38;2;94;100;94;48;2;30;152;62m🙃",
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
				theme:      themeOrDefault("green"),
				cols:       tt.cols,
				visCols:    tt.visCols,
				denom:      tt.denom,
				termWidth:  tt.cols,
				isTerminal: true,
				isColored:  tt.isColored,
			}
			ws.writeString(&buf, tt.str)
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
			want:         "\033[38;2;255;255;255;48;2;10;25;12mp" + "\033[38;2;253;253;253;48;2;10;27;12mr" +
			              "\033[38;2;250;250;250;48;2;10;29;13mo" + "\033[38;2;247;247;247;48;2;11;31;14mc" +
			              "\033[38;2;244;244;244;48;2;11;34;15me" + "\033[38;2;241;241;241;48;2;11;36;16ms" +
			              "\033[38;2;238;239;238;48;2;12;38;17ms" + "\033[38;2;235;236;235;48;2;12;41;18mi" +
			              "\033[38;2;232;233;232;48;2;13;43;19mn" + "\033[38;2;229;230;229;48;2;13;45;20mg" +
			              "\033[38;2;226;227;226;48;2;13;48;21m " + "\033[38;2;223;224;223;48;2;14;50;22m(" +
			              "\033[38;2;220;222;220;48;2;14;52;23m " + "\033[38;2;217;219;217;48;2;14;55;23m7" +
			              "\033[38;2;214;216;214;48;2;15;57;24m7" + "\033[38;2;211;213;211;48;2;15;59;25m%" +
			              "\033[38;2;208;210;208;48;2;16;62;26m)" + "\033[38;2;205;207;205;48;2;16;64;27m:" +
			              "\033[38;2;202;204;202;48;2;16;66;28m " + "\033[38;2;199;201;199;48;2;17;69;29mw" +
			              "\033[38;2;196;199;196;48;2;17;71;30mo" + "\033[38;2;193;196;193;48;2;17;74;31mr" +
			              "\033[38;2;190;193;190;48;2;18;76;32mk" + "\033[38;2;187;190;187;48;2;18;78;33mi" +
			              "\033[38;2;184;187;184;48;2;19;81;34mn" + "\033[38;2;181;184;181;48;2;19;83;35mg" +
			              "\033[38;2;178;181;178;48;2;19;85;36m." + "\033[38;2;175;179;175;48;2;20;88;36m." +
			              "\033[38;2;172;176;172;48;2;20;90;37m." +
			              "\033[30;48;2;21;92;38m "  + "\033[30;48;2;21;95;39m "  + "\033[30;48;2;21;97;40m "  +
			              "\033[30;48;2;22;99;41m "  + "\033[30;48;2;22;102;42m " + "\033[30;48;2;22;104;43m " +
			              "\033[30;48;2;23;106;44m " + "\033[30;48;2;23;109;45m " + "\033[30;48;2;24;111;46m " +
			              "\033[30;48;2;24;113;47m " + "\033[30;48;2;24;116;47m " + "\033[30;48;2;25;118;48m " +
			              "\033[30;48;2;25;120;49m " + "\033[30;48;2;25;123;50m " + "\033[30;48;2;26;125;51m " +
			              "\033[30;48;2;26;127;52m " + "\033[30;48;2;27;130;53m " + "\033[30;48;2;27;132;54m " +
			              "\033[30;48;2;27;134;55m " + "\033[30;48;2;28;137;56m " + "\033[30;48;2;28;139;57m " +
			              "\033[30;48;2;28;141;58m " + "\033[30;48;2;29;144;59m " + "\033[30;48;2;29;146;60m " +
			              "\033[30;48;2;30;148;60m " + "\033[30;48;2;30;151;61m " + "\033[30;48;2;30;153;62m " +
			              "\033[30;48;2;31;155;63m " + "\033[30;48;2;31;158;64m " + "\033[30;48;2;32;160;65m " +
			              "\033[30;48;2;32;163;66m " + "\033[30;48;2;32;165;67m " + "\033[0m\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Progress{
				tracker:        getTracker(Standard, 3),
				output:         io.Discard,
				isTerminal:     tt.isTerminal,
				isTerminalFunc: isTerminal,
				theme:          themeOrDefault("green"),
			}
			p.prepareTerminal()
			p.state.Store(pack(t, 80, 0))

			buf := make([]byte, 0, p.layout.bufCap(int(p.state.Load() >> 16)))
			_ = p.writeStatus(&buf, tt.pctSigDigits, tt.status, false)

			if diff := cmp.Diff(tt.want, p.lastRenderedFrame()); diff != "" {
				t.Errorf("writeStatus(%d, %q, %t) mismatch (-want +got):\n%s", tt.pctSigDigits, tt.status, false, diff)
			}
		})
	}
}
