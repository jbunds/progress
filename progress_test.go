package progress

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unique"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var opts = cmp.Options{
	cmpopts.EquateEmpty(),
	cmpopts.IgnoreFields(Progress{}, // non-trivial to compare
		"buf",       "output",     "stopChan",
		"doneChan",  "resizeChan", "resizeHandler",
		"closeOnce", "drawNotify", "isTerminalFunc"),
	cmp.AllowUnexported(
		Progress{},        realClock{},     layout{},
		standardTracker{}, uniqueTracker{},
		percentTracker{},  fractionTracker{}),
	cmpopts.EquateComparable(
		atomic.Value{},
		atomic.Uint32{},
		atomic.Uint64{}),
	cmp.FilterValues(func(x, _ any) bool { // recursively unwraps atomic types to facilitate deep comparison of underlying values
		_, ok := x.(interface{ Load() any })
		if !ok && reflect.ValueOf(x).CanAddr() {
			_, ok = reflect.ValueOf(x).Addr().Interface().(interface{ Load() any })
		}
		return ok
	}, cmp.Transformer("unwrapAtomic", func(x any) any {
		if loader, ok := x.(interface{ Load() any }); ok { return loader.Load() }
		v := reflect.ValueOf(x)
		if v.CanAddr() { return v.Addr().Interface().(interface{ Load() any }).Load() }
		return x
	})),
}

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		totalUnits  uint64
		opts        []Option
		wantTotal   uint64
		wantTracker statusTracker
	}{
		{
			name:        "weight-based accumulation",
			totalUnits:  100,
			wantTotal:   100,
			wantTracker: getTracker(Standard, 0),
		},
		{
			name:        "fractional path allocation",
			totalUnits:  0,
			wantTotal:   0,
			wantTracker: getTracker(Standard, 0),
		},
		{
			name:        "verify overflow safety",
			totalUnits:  scale + 1000,
			wantTotal:   scale,
			wantTracker: getTracker(Standard, 0),
		},
		{
			name:        "verify WithTracker",
			totalUnits:  0,
			wantTotal:   0,
			opts:        []Option{WithTracker(Unique)},
			wantTracker: getTracker(Unique, 0),
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
		})
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
		want   uint16
	}{
		{
			name:  "falls back to minWidth for non-terminal files",
			output: w, // pipes have no width
			want:   uint16(minWidth),
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
			ctx := t.Context()
			p   := New(ctx, 0, io.Discard)
			p.AddTotal(tt.units)
			t.Cleanup(func() { p.Close() })
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

func TestReportContention(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(runtime.NumCPU()))
	p := &Progress{ tracker: getTracker(Standard, 100) }
	p.total.Store(100)
	var wg sync.WaitGroup
	for i := range 200 {
		wg.Go(func() {
			for j := range 500 {
				p.Report(float64(1e12) + float64(i * j), "spamming Report to trigger contention inside CAS loops")
				if j % 50 == 0 { p.current.Store(0) }
			}
		})
	}
	wg.Wait()
	want := uint32(10000)
	if diff := cmp.Diff(want, p.state.Load()); diff != "" {
		t.Errorf("p.state.Load() mismatch (-want +got):\n%s", diff)
	}
}

func pack(termWidth int, percent float64) uint32 {
	return uint32(termWidth & 0xFFFF) << 16 | uint32(percent * 10000)
}

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

func TestRenderLoop(t *testing.T) {
	t.Parallel()

	got         := new(bytes.Buffer)
	tickTrigger := make(chan time.Time, 1)
	notify      := make(chan struct{}, 1) // awaits the completion of a draw cycle, buffered to prevent deadlocks

	p := &Progress{
		tracker:    getTracker(Standard, 0),
		output:     got,
		clock:      &fakeClock{ c: tickTrigger },
		drawNotify: notify,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
	}

	tickAndExpectDraw := func() {
		tickTrigger <- time.Now()
		<-notify // draw() cycle completed
	}

	tickAndExpectSkip := func() {
		tickTrigger <- time.Now()
		for p.lastState.Load() != p.state.Load() { // wait until renderLoop has processed the state
			runtime.Gosched() // yield the processor to allow the scheduler to run the renderLoop goroutine so it completes the atomic state update
		}
		select {       // verify that the notify channel is empty
		case <-notify: // unexpected draw() cycle completed
			t.Errorf("redundant draw rendered")
		default:
		}
	}

	p.total.Store(100)
	p.state.Store(pack(minWidth, 0))

	go p.renderLoop(t.Context())

	p.Report(10, "working...")
	tickAndExpectDraw()

	p.Report(0, "working...") // redundant report
	tickAndExpectSkip()

	t.Cleanup(func() { p.Close() })
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

func TestClose(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		total    uint64
		err      error
		wantOut  string
		wantProg *Progress
	}{
		{
			name:    "succeeds",
			total:   100,
			wantOut: "processing (100%): done\n",
			wantProg: &Progress{
				tracker: getTracker(Standard, 100),
				clock:   &realClock{ dur: 16 * time.Millisecond },
			},
		},
		{
			name:     "progress tracking was aborted",
			total:    200,
			err:      errors.New("aborted for some reason"),
			wantOut:  "stopped (aborted for some reason)\n",
			wantProg: &Progress{
				tracker: getTracker(Standard, 200),
				clock:   &realClock{ dur: 16 * time.Millisecond },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			got         := new(bytes.Buffer)
			p           := New(ctx, tt.total, got)
			tt.wantProg.total.Store(tt.total)

			cancel(tt.err)
			p.Close()
			<-p.doneChan

			finalState := p.state.Load() // capture final state
			tt.wantProg.state.Store(     // sync tt.wantProg to match final state
				pack(int(finalState >> 16), float64(finalState & 0xFFFF) / 10000))

			if diff := cmp.Diff(tt.wantOut, got.String()); diff != "" {
				t.Errorf("Close(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantProg, p, opts...); diff != "" {
				t.Errorf("Close(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestHandleResize(t *testing.T) {
	t.Parallel()

	fakeClock := &fakeClock{ c: make(chan time.Time, 1) }
	notify    := make(chan struct{}, 1)

	mockTermWidth     := uint16(minWidth)
	mockResizeHandler := func() uint16 { return mockTermWidth }

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	p := New(ctx, 0, io.Discard, withClock(fakeClock), withResizeHandler(mockResizeHandler))
	p.drawNotify    = notify // drawNotify signals the completion of a draw cycle in the renderLoop
	p.resizeHandler = mockResizeHandler

	mockTermWidth = 120
	p.resizeChan <-syscall.SIGWINCH

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
	want := uint16(p.tracker.layout().staticWidth & 0xFFFF)
	got  := p.getResizedTermWidth()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("getResizedTermWidth() mismatch (-want +got):\n%s", diff)
	}
}

func TestPrepareTerminal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		isTerminal bool
	}{
		{
			name:       "terminal detected",
			isTerminal: true,
		},
		{
			name:       "not a terminal",
			isTerminal: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Progress{
				output:         os.Stderr,
				tracker:        getTracker(Standard, 0),
				isTerminalFunc: func(any) bool { return tt.isTerminal },
			}
			p.prepareTerminal()
			layout := p.tracker.layout()
			if tt.isTerminal {
				if diff := cmp.Diff("\r\033[2K\r", layout.clearSeq); diff != "" {
					t.Errorf("prepareTerminal(%q) clearSeq mismatch (-want +got):\n%s", tt.name, diff)
				}
				if diff := cmp.Diff("\r\033[?25h", layout.doneSeq); diff != "" {
					t.Errorf("prepareTerminal(%q) doneSeq mismatch (-want +got):\n%s", tt.name, diff)
				}
				if diff := cmp.Diff("", layout.lineTerminator); diff != "" {
					t.Errorf("prepareTerminal(%q) lineTerminator mismatch (-want +got):\n%s", tt.name, diff)
				}
			} else {
				if diff := cmp.Diff("", layout.clearSeq); diff != "" {
					t.Errorf("prepareTerminal(%q) clearSeq mismatch (-want +got):\n%s", tt.name, diff)
				}
				if diff := cmp.Diff("\n", layout.doneSeq); diff != "" {
					t.Errorf("prepareTerminal(%q) doneSeq mismatch (-want +got):\n%s", tt.name, diff)
				}
				if diff := cmp.Diff("\n", layout.lineTerminator); diff != "" {
					t.Errorf("prepareTerminal(%q) lineTerminator mismatch (-want +got):\n%s", tt.name, diff)
				}
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want bool
	}{
		{
			name: "succeeds",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Progress{
				output:         os.Stderr,
				isTerminalFunc: func(v any) bool { return isTerminalInternal(v, false, false) },
			}
			got := p.isTerminalFunc(p.output)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("isTerminalInternal(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestGetFdOfNonFile(t *testing.T) {
	t.Parallel()
	w    := "foo"
	got  := getFD(w)
	want := -1
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("getFD(%q) mismatch (-want +got):\n%s", w, diff)
	}
}
