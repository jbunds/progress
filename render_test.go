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
		statusText string
		want       string
	}{
		{
			name:       "nominal terminal width of 80", // minWidth == 80
			state:      pack(80, 0.47),                 // 80 - len("processing (100%): ") == 61
			statusText: "just a small fish in a big sea",
			want:       "processing ( 47%): just a small fish in a big sea\n",
		},
		{
			name:       "status message truncated from the left and prepended with an ellipsis",
			state:      pack(40, 0.71), // 40 - len("processing (100%): ") == 21
			statusText: "this is a very long status message that must be truncated",
			want:       "processing ( 71%): …at must be truncated\n",
		},
		{
			name:       "status message truncated from the left with no ellipsis prepended (terminal too narrow)",
			state:      pack(22, 0.93), // 22 - len("processing (100%): ") == 3
			statusText: "short message",
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
			p.layout = p.tracker.baseLayout()
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
			p.layout = p.tracker.baseLayout()
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
			p.layout = p.tracker.baseLayout()
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
	p.layout = p.tracker.baseLayout()
	p.buf.Store(buf())

	p.total.Store(73)
	p.state.Store(pack(minWidth, 0))

	go p.renderLoop(t.Context())
	t.Cleanup(func() { p.Close() })

	p.Report(11, "completed 11 units of work") // first report: 11/73
	tickTrigger <-time.Now()
	<-notify

	tickTrigger <-time.Now() // should skip redundant redraw

	want := "processing ( 15%): 11/73\n"
	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("renderLoop() mismatch (-want +got):\n%s", diff)
	}

	got.Reset()

	p.Report(34, "completed another 34 units of work") // second report: 45/73
	tickTrigger <-time.Now()
	<-notify

	want = "processing ( 62%): 45/73\n"
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
		wantBuf       []byte
		wantIsColored bool
	}{
		{
			name:    "wide load",
			cols:    30,
			denom:   30 - 1,
			visCols: 20,
			str:     "🙃",
			wantBuf:       []byte("\033[38;2;94;100;94;48;2;30;152;62m🙃"),
			wantIsColored: true,
		},
		{
			name:      "reset",
			cols:      30,
			denom:     30 - 1,
			visCols:   30,
			isColored: true,
			str:       "foo",
			wantBuf:   []byte("\033[0mfoo"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &writeState{
				buf:       make([]byte, 0),
				isTerm:    true,
				termWidth: tt.cols,
				cols:      tt.cols,
				denom:     tt.denom,
				visCols:   tt.visCols,
				isColored: tt.isColored,
				theme:     themeOrDefault("green"),
			}
			s.writeString(tt.str)
			if diff := cmp.Diff(tt.wantBuf, s.buf); diff != "" {
				t.Errorf("writeString(%q) mismatch (-want +got):\n%s", tt.str, diff)
			}
			if diff := cmp.Diff(tt.wantIsColored, s.isColored); diff != "" {
				t.Errorf("writeString(%q) mismatch (-want +got):\n%s", tt.str, diff)
			}
		})
	}
}

func TestWriteStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		pctSigDigits uint16
		status       string
		trunc        bool
		want         string
		wantErr      bool
	}{
		{
			name:         "succeeds",
			pctSigDigits: 10000,
			status:       "working...",
			want:         "\033[38;2;255;255;255;48;2;10;25;12mp"  +
			              "\033[38;2;248;248;248;48;2;11;31;14mr"  +
			              "\033[38;2;240;240;240;48;2;12;37;16mo"  +
			              "\033[38;2;231;232;231;48;2;13;44;19mc"  +
			              "\033[38;2;223;225;223;48;2;14;50;22me"  +
			              "\033[38;2;215;217;215;48;2;15;56;24ms"  +
			              "\033[38;2;207;209;207;48;2;16;63;27ms"  +
			              "\033[38;2;199;201;199;48;2;17;69;29mi"  +
			              "\033[38;2;191;194;191;48;2;18;75;32mn"  +
			              "\033[38;2;183;186;183;48;2;19;82;34mg"  +
			              "\033[38;2;175;178;175;48;2;20;88;37m "  +
			              "\033[38;2;166;170;166;48;2;21;95;39m("  +
			              "\033[38;2;158;163;158;48;2;22;101;42m1" +
			              "\033[38;2;150;155;150;48;2;23;107;44m0" +
			              "\033[38;2;142;147;142;48;2;24;114;47m0" +
			              "\033[38;2;134;139;134;48;2;25;120;49m%" +
			              "\033[38;2;126;132;126;48;2;26;126;52m)" +
			              "\033[38;2;118;124;118;48;2;27;133;54m:" +
			              "\033[38;2;110;116;110;48;2;28;139;57m " +
			              "\033[38;2;102;108;102;48;2;29;146;59mw" +
			              "\033[38;2;94;100;94;48;2;30;152;62mo"   +
			              "\033[38;2;85;93;85;48;2;31;158;64mr"    +
			              "\033[38;2;77;85;77;48;2;32;165;67mk"    +
			              "\033[38;2;69;77;69;48;2;33;171;69mi"    +
			              "\033[38;2;61;69;61;48;2;34;177;72mn"    +
			              "\033[38;2;53;62;53;48;2;35;184;74mg"    +
			              "\033[38;2;45;54;45;48;2;36;190;77m."    +
			              "\033[38;2;37;46;37;48;2;37;197;79m."    +
			              "\033[38;2;29;38;29;48;2;38;203;82m."    +
			              "\033[30;48;2;40;210;85m \033[0m\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := new(bytes.Buffer)
			p   := &Progress{
				tracker:    getTracker(Standard, 3),
				output:     got,
				isTerminal: true,
				theme:      themeOrDefault("green"),
			}
			p.layout = p.tracker.baseLayout()
			p.buf.Store(buf())
			p.state.Store(uint32(30) << 16)
			_ = p.writeStatus(tt.pctSigDigits, tt.status, tt.trunc)
			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("writeStatus(%d, %q, %t) mismatch (-want +got):\n%s", tt.pctSigDigits, tt.status, tt.trunc, diff)
			}
		})
	}
}
