package progress

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"reflect"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func getCmpOpts() cmp.Options {
	return cmp.Options{
		cmpopts.IgnoreFields(Progress{}, // non-trivial to compare or irrelevant in tests
			"theme",      "output",     "fgColor",    "bufPool",
			"stopChan",   "doneChan",   "lastFrame",  "closeOnce", 
			"persistBar", "resizeChan", "drawNotify", "isTerminal", "resizeHandler"),
		cmp.AllowUnexported(
			Progress{},        realClock{},
			layout{},          rgb{},
			standardTracker{}, uniqueTracker{},
			percentTracker{},  fractionTracker{}),
		cmpopts.IgnoreUnexported(theme{}),
		cmpopts.EquateComparable(
			atomic.Value{},
			atomic.Uint32{},
			atomic.Uint64{},
			atomic.Pointer[string]{}),
		cmp.FilterValues(func(x, _ any) bool { // recursively unwraps atomic types to facilitate deep comparison of underlying values
			_, ok := x.(interface{ Load() any })
			if !ok && reflect.ValueOf(x).CanAddr() {
				_, ok = reflect.ValueOf(x).Addr().Interface().(interface{ Load() any })
			}
			return ok
		}, cmp.Transformer("unwrapAtomic", func(x any) any {
			if loader, ok := x.(interface{ Load() any }); ok {
				return loader.Load()
			}
			v := reflect.ValueOf(x)
			if v.CanAddr() {
				if loader, ok := v.Addr().Interface().(interface{ Load() any }); ok {
					return loader.Load()
				}
			}
			return x
		})),
	}
}

// convenience / readability test helper to allow percentages to be
// expressed as a decimal fraction of 100, e.g., 12.3456% == 0.123456.
func pack(t *testing.T, termWidth int, percent float64) uint32 {
	t.Helper()
	return uint32(termWidth & 0xFFFF) << 16 | uint32(percent * 10000)
}

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		totalUnits  uint64
		opts        []Option
		wantTotal   uint64
		wantTracker statusTracker
		wantTheme   *theme
	}{
		{
			name:        "weight-based accumulation",
			totalUnits:  100,
			wantTotal:   100,
			wantTracker: getTracker(Standard, 0),
			wantTheme:   newThemeRegistry().get("sunset"),
		},
		{
			name:        "fractional path allocation",
			totalUnits:  0,
			wantTotal:   0,
			wantTracker: getTracker(Standard, 0),
			wantTheme:   newThemeRegistry().get("sunset"),
		},
		{
			name:        "verify overflow safety",
			totalUnits:  scale + 1000,
			wantTotal:   scale,
			wantTracker: getTracker(Standard, 0),
			wantTheme:   newThemeRegistry().get("sunset"),
		},
		{
			name:        "verify WithTracker, WithTheme, and WithPersistBar",
			totalUnits:  0,
			wantTotal:   0,
			opts:        []Option{WithTracker(Unique), WithTheme("rainbow"), WithPersistBar(true)},
			wantTracker: getTracker(Unique, 0),
			wantTheme:   newThemeRegistry().get("rainbow"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			buf := new(bytes.Buffer)
			got := New(ctx, tt.totalUnits, buf, tt.opts...)
			t.Cleanup(func() { got.Close() })
			if diff := cmp.Diff(tt.wantTotal, got.total.Load()); diff != "" {
				t.Errorf("New(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if got.stopChan == nil || got.doneChan == nil || got.resizeChan == nil {
				t.Errorf("one or more channels were not initialized")
			}
			if diff := cmp.Diff(tt.wantTracker, got.tracker); diff != "" {
				t.Errorf("New(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantTheme, got.theme); diff != "" {
				t.Errorf("New(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestInitialBudget(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want float64
	}{
		{
			name: "succeeds",
			want: float64(scale),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			p   := New(ctx, 0, io.Discard)
			t.Cleanup(func() { p.Close() })
			got := p.InitialBudget()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("InitialBudget(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestAddTotal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		units uint64
		want  uint64
	}{
		{
			name:  "less than scale added",
			units: 100,
			want:  100,
		},
		{
			name:  "more than scale added; total capped at scale",
			units: scale + 1000,
			want:  scale,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			p   := New(ctx, 0, io.Discard)
			t.Cleanup(func() { p.Close() })
			p.AddTotal(tt.units)
			if diff := cmp.Diff(tt.want, p.total.Load()); diff != "" {
				t.Errorf("AddTotal(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestReport(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		total     uint64
		unitsDone float64
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
			total:     scale,
			unitsDone: 1,
			status:    "completed 1 unit of work",
			want:      (1 * scale / scale) * 3,
		},
		{
			name:      "weight-based; reported work done cannot exceed total work",
			total:     100,
			unitsDone: 150,   // report 150% of total work done
			status:    "completed more work than budgeted",
			want:      scale, // budget not exceeded
		},
		{
			name:      "fractional path allocation; direct accumulation",
			total:     0,
			unitsDone: float64(scale) / 10,
			status:    "1/10 of the total work done",
			want:      (scale / 10) * 3,
		},
		{
			name:      "fractional path allocation; verify safe accumulation of a very large amount of work",
			total:     0,
			unitsDone: math.MaxUint64 / 4, // ~4612 times larger than scale (1e15)
			status:    "very large amount of work done",
			want:      scale,              // budget not exceeded
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			p   := New(ctx, tt.total, io.Discard)
			t.Cleanup(func() { p.Close() })
			for range 3 { p.Report(tt.unitsDone, tt.status) }
			if diff := cmp.Diff(tt.want, p.current.Load()); diff != "" {
				t.Errorf("current progress was not updated (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRenderLoop(t *testing.T) {
	t.Parallel()

	tickTrigger := make(chan time.Time, 2) // capacity of 2 to cover the post-loop drain case
	notify      := make(chan struct{},  1) // awaits the completion of a draw cycle, buffered to prevent deadlocks

	p := &Progress{
		tracker:    getTracker(Standard, 100),
		output:     io.Discard,
		isTerminal: isTerminal,
		clock:      fakeClock{ c: tickTrigger },
		drawNotify: notify,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
	}
	p.resizeHandler = p.getResizedTermWidth

	p.initBufPool()

	tickAndExpectDraw := func() {
		tickTrigger <-time.Time{}
		<-notify // draw() cycle completed
	}

	tickAndExpectSkip := func() {
		beforeTickFrame := p.lastFrameRendered()

		tickTrigger <-time.Time{}
		<-notify

		afterTickFrame := p.lastFrameRendered()

		if beforeTickFrame != afterTickFrame {
			t.Errorf("redundant frame rendered:\n%q", afterTickFrame)
		}
	}

	ctx, cancel := context.WithCancel(t.Context())

	go p.renderLoop(ctx)

	p.Report(10, "working...")
	tickAndExpectDraw()

	p.Report(0, "working...") // redundant report
	tickAndExpectSkip()

	// avert thine eyes, for the remainder of this test just ekes out gratuitous test coverage...

	p.Report(20, "post-loop drain case")
	tickTrigger <- time.Time{} // send a tick to wake up the loop select block
	tickTrigger <- time.Time{} // send a tick to wake up the loop select block
	<-notify                   // wait for the sync logic to execute and notify
	<-notify                   // wait for the post-loop drain to catch the second tick

	cancel()                   // cover the first select block's ctx.Done() case

	<-p.doneChan               // wait for the first renderLoop goroutine to exit

	select {
	case <-notify:             // drain the notify channel
	default:
	}

	p.stopChan   = make(chan struct{})
	p.doneChan   = make(chan struct{})
	resizeSig   := make(chan os.Signal, 1)
	p.resizeChan = resizeSig // hijack resizeChan to ensure the p.Close() call is caught by the second select block

	go p.renderLoop(t.Context())

	resizeSig <- syscall.SIGWINCH // send a SIGWINCH signal to the hijacked resizeChan
	<-notify                      // wait for renderLoop to handle the resize signal

	p.Close()    // cover the second select block's stopChan case
	<-p.doneChan // wait for the second renderLoop goroutine to exit
}

func TestClose(t *testing.T) {
	t.Parallel()
	isTerminal    := func(any) bool { return true }
	coloredOutput :=
		"\033[38;2;255;255;255;48;2;48;25;52mp"  + "\033[38;2;255;255;255;48;2;54;24;52mr"  +
		"\033[38;2;255;255;255;48;2;59;23;52mo"  + "\033[38;2;255;255;255;48;2;65;22;53mc"  +
		"\033[38;2;255;255;255;48;2;71;21;53me"  + "\033[38;2;255;255;255;48;2;77;20;53ms"  +
		"\033[38;2;255;255;255;48;2;82;19;53ms"  + "\033[38;2;255;255;255;48;2;88;18;53mi"  +
		"\033[38;2;255;255;255;48;2;94;17;54mn"  + "\033[38;2;255;255;255;48;2;100;16;54mg" +
		"\033[38;2;255;255;255;48;2;105;16;54m " + "\033[38;2;255;255;255;48;2;111;15;54m(" +
		"\033[38;2;255;255;255;48;2;117;14;54m1" + "\033[38;2;255;255;255;48;2;123;13;54m0" +
		"\033[38;2;255;255;255;48;2;128;12;55m0" + "\033[38;2;255;255;255;48;2;134;11;55m%" +
		"\033[38;2;255;255;255;48;2;140;10;55m)" + "\033[38;2;255;255;255;48;2;145;9;55m:"  +
		"\033[38;2;255;255;255;48;2;151;8;55m "  + "\033[38;2;255;255;255;48;2;157;7;56md"  +
		"\033[38;2;255;255;255;48;2;163;6;56mo"  + "\033[38;2;255;255;255;48;2;168;5;56mn"  +
		"\033[38;2;255;255;255;48;2;174;4;56me"  +
		"\033[48;2;180;3;56m "   + "\033[48;2;186;2;57m "   + "\033[48;2;191;1;57m "   +
		"\033[48;2;197;0;57m "   + "\033[48;2;200;2;57m "   + "\033[48;2;203;6;57m "   +
		"\033[48;2;205;9;56m "   + "\033[48;2;207;12;56m "  + "\033[48;2;209;15;56m "  +
		"\033[48;2;211;19;56m "  + "\033[48;2;213;22;55m "  + "\033[48;2;215;25;55m "  +
		"\033[48;2;217;29;55m "  + "\033[48;2;220;32;55m "  + "\033[48;2;222;35;55m "  +
		"\033[48;2;224;39;54m "  + "\033[48;2;226;42;54m "  + "\033[48;2;228;45;54m "  +
		"\033[48;2;230;48;54m "  + "\033[48;2;232;52;53m "  + "\033[48;2;234;55;53m "  +
		"\033[48;2;237;58;53m "  + "\033[48;2;239;62;53m "  + "\033[48;2;241;65;53m "  +
		"\033[48;2;243;68;52m "  + "\033[48;2;245;72;52m "  + "\033[48;2;247;75;52m "  +
		"\033[48;2;249;78;52m "  + "\033[48;2;251;81;51m "  + "\033[48;2;254;85;51m "  +
		"\033[48;2;255;88;50m "  + "\033[48;2;255;92;48m "  + "\033[48;2;255;97;46m "  +
		"\033[48;2;255;101;45m " + "\033[48;2;255;105;43m " + "\033[48;2;255;109;41m " +
		"\033[48;2;255;113;39m " + "\033[48;2;255;117;37m " + "\033[48;2;255;121;35m " +
		"\033[48;2;255;125;33m " + "\033[48;2;255;129;31m " + "\033[48;2;255;133;29m " +
		"\033[48;2;255;138;27m " + "\033[48;2;255;142;25m " + "\033[48;2;255;146;23m " +
		"\033[48;2;255;150;21m " + "\033[48;2;255;154;19m " + "\033[48;2;255;158;17m " +
		"\033[48;2;255;162;15m " + "\033[48;2;255;166;14m " + "\033[48;2;255;170;12m " +
		"\033[48;2;255;174;10m " + "\033[48;2;255;179;8m "  + "\033[48;2;255;183;6m "  +
		"\033[48;2;255;187;4m "  + "\033[48;2;255;191;2m "  + "\033[48;2;255;195;0m "
	tests := []struct {
		name       string
		total      uint64
		opts       []Option
		err        error
		wantOut    string
	}{
		{
			name:    "with WithPersistBar(false)",
			total:   100,
			opts:    []Option{WithIsTerminalFunc(isTerminal)},
			wantOut: ansiHideCursor + ansiClearSeq       + coloredOutput +
			         ansiResetAttrs + ansiLineTerminator + ansiClearSeq  + ansiDoneSeq,
		},
		{
			name:    "with WithPersistBar(true)",
			total:   200,
			opts:    []Option{WithIsTerminalFunc(isTerminal), WithPersistBar(true)},
			wantOut: ansiHideCursor + ansiClearSeq       + coloredOutput +
			         ansiResetAttrs + ansiLineTerminator + "\n"          + ansiDoneSeq,
		},
		{
			name:    "aborted",
			total:   300,
			opts:    []Option{WithIsTerminalFunc(isTerminal)},
			err:     errors.New("aborted for some reason"),
			wantOut: ansiHideCursor + ansiClearSeq + "stopped (aborted for some reason)" + ansiDoneSeq,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			t.Cleanup(func() { cancel(nil) })

			got := new(bytes.Buffer)
			p   := New(ctx, tt.total, got, tt.opts...)

			wantProg := &Progress{
				tracker: getTracker(Standard, tt.total),
				clock:   &realClock{ dur: 16 * time.Millisecond },
				layout:  p.layout,
			}
			wantProg.total.Store(tt.total)

			if tt.err != nil { cancel(tt.err) }
			p.Close()
			<-p.doneChan

			finalState := p.state.Load() // capture final state
			wantProg.state.Store(        // sync wantProg to match final state
				pack(t, int(finalState >> 16), float64(finalState & 0xFFFF) / 10000))

			if diff := cmp.Diff(tt.wantOut, got.String()); diff != "" {
				t.Errorf("Close(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(wantProg, p, getCmpOpts()); diff != "" {
				t.Errorf("Close(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
