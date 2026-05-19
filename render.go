package progress

import (
	"unique"
	"unsafe"
)

// ANSI escape sequences
//
//   https://en.wikipedia.org/wiki/ANSI_escape_code#24-bit
//   https://en.wikipedia.org/wiki/ANSI_escape_code#Select_Graphic_Rendition_parameters
//   https://ecma-international.org/publications-and-standards/standards/ecma-48/
//
//   │ atom or sequence  │ description
//   ├───────────────────┼───────────────────────────────────────────────────────────────
//   │ \033              │ ESC character (octal)
//   │ \033[             │ CSI (control sequence introducer)
//   │ ?                 │ extended terminal mode prefix (per DEC / xterm specification)
//   │                   │
//   │ \033[K            │ EL (Erase in Line): erase from cursor position to end of line
//   │ \033[0m           │ SGR 0 (select graphic rendition: reset):
//   │                   │     0: deactivate all character attributes, styles, and colors
//   │                   │     m: SGR terminator)
//   │                   │
//   │ \033[30;48;2;     │ SGR sequence:
//   │                   │     30: set foreground (text) color to black
//   │                   │     48: set background...
//   │                   │      2: ...per incoming 24-bit RGB triplet
//   │ \033[38;2;        │ SGR sequence:
//   │                   │     38: set foreground (text) color...
//   │                   │      2: ...per incoming 24-bit RGB triplet
//   │                   │
//   │ \033[?25h         │ DECTCEM (DEC Text Cursor Enable Mode) high: show the cursor
//   │ \033[?25l         │ DECTCEM (DEC Text Cursor Enable Mode)  low: hide the cursor
//   │                   │
//   │   2026            │ synchronized updates mode designation number
//   │ [?2026h           │ synchronized output high:   activate synchronized output mode
//   │ [?2026l           │ synchronized output  low: deactivate synchronized output mode

const (
	hideCursor     = "\033[?25l"                // hide cursor
	clearSeq       = "\r\033[?2026h"            // move cursor to beginning of line; activate synchronized output mode
	doneSeq        = "\033[0m\r\033[?25h"       // reset all attributes to defaults; move cursor to beginning of line; restore cursor
	lineTerminator = "\033[K\033[0m\033[?2026l" // erase from cursor position to end of line; reset all attributes; deactivate synchronized output mode
	prepColorSeq   = "\033[30;48;2;"            // set foreground (text) color to black (30); set background color (48) per incoming 24-bit RGB triplet (2)
	setFgColor     = "\033[38;2;"               // set foreground (text) color (38) per incoming 24-bit RGB triplet (2)
	prepSetBgColor = ";48;2;"                   // prepare to set background color (48) per incoming 24-bit RGB triplet
	resetAttr      = "\033[0m"                  // reset all attributes to defaults
)

// zero-overhead production / default no-op stubs:
// - decoupled from the production runtime path via compiler optimization.
// - overridden during test execution per init_test.go.
// - enables instrumentation hooks to be injected to ensure synchronous, deterministic tests.
var (
	syncCompleteHook   = func(*Progress         ) {}
	storeLastFrameHook = func(*Progress, *[]byte) {}
)

// writeState encapsulates canvas boundaries, color theme, and cursor positions
// across sequential draws in a stack-allocated block to prevent heap escaping.
type writeState struct {
	theme      *theme
	cols       int
	visCols    int
	denom      int
	termWidth  int
	isTerminal bool
	isColored  bool
}

// sync performs a state-aware redraw, skipping redundant redraws if the
// progress and status values haven't changed since the last render.
func (p *Progress) sync(buf *[]byte) {
	defer func() { syncCompleteHook(p) }()

	currentState := p.state.Load()
	lastState    := p.lastState.Load()
	lastVal      := p.lastStatusVal.Load()

	// TODO(jeff): fix the leaky tracker abstraction originally designed to provide transparent polymorphism
	var currentVal uint64
	switch t := p.tracker.(type) {
	case *standardTracker:
		if ptr := t.status.Load(); ptr != nil {
			// #nosec G103 -- audited: stable heap pointer address used strictly as an identity token
			currentVal = uint64(uintptr(unsafe.Pointer(ptr)))
		}
	case *uniqueTracker:
		if val, ok := t.status.Load().(unique.Handle[string]); ok {
			// #nosec G103 -- audited: extracting underlying internal handle pointer for stable identity comparison
			currentVal = uint64(*(*uintptr)(unsafe.Pointer(&val)))
		}
	case *fractionTracker:
		currentVal = t.status.Load()
	case *percentTracker:
		currentVal = 0
	}

	if currentState == lastState &&
	   currentVal   == lastVal { return } // status unchanged; skip redundant redraw

	p.draw(buf, currentState)

	p.lastState.Store(currentState)
	p.lastStatusVal.Store(currentVal)
}

// draw formats and renders the current progress status to the terminal,
// truncating text as needed to fit within the terminal width.
func (p *Progress) draw(buf *[]byte, state uint32) {
	maxLen    := state >> 16 - uint32(p.layout.staticWidth & 0xFFFF)
	status    := ""
	truncated := false

	if maxLen > 0 {
		status, truncated = truncateFromLeft(p.tracker.load(), int(maxLen)) // truncate from left to show most relevant portion (e.g., file basename)
	}

	_ = p.writeStatus(buf, state & 0xFFFF, status, truncated)
}

// lastFrameRendered returns the last rendered frame string.
func (p *Progress) lastFrameRendered() string {
	if v := p.lastFrame.Load(); v != nil { return *v }
	return ""
}

// writeStatus writes the progress status to to p.output (nominally os.Stderr) per an atomic system call.
func (p *Progress) writeStatus(buf *[]byte, pctSigDigits uint32, status string, truncated bool) error {
	*buf = append(*buf, p.layout.clearSeq...)

	termWidth := int(p.state.Load() >> 16)

	// cache loop-invariant bar gradient values
	//
	// global gradient: maps the color spectrum across the full terminal width.
	//                  the color of any character depends strictly on its absolute screen column.
	var denom int
	if termWidth > 1 { denom = termWidth - 1 }

	ws := writeState{
		theme:      p.theme,
		cols:       (termWidth * int(pctSigDigits)) / 10000,
		visCols:    0,
		denom:      denom,
		termWidth:  termWidth,
		isTerminal: p.isTerminal(p.output),
		isColored:  false,
	}

//	// cache loop-invariant bar gradient values
//	//
//	// dynamic bar gradient: stretches the full color spectrum to fit the active bar width.
//	//                       the color gradient transitions completely from 0% to 100% inside the filled bar.
//	barCols := (termWidth * int(pctSigDigits)) / 10000
//	var denom int
//	if barCols > 1 { denom = barCols - 1 }
//
//	ws := writeState{
//		theme:      p.theme,
//		cols:       barCols,
//		visCols:    0,
//		denom:      denom,
//		termWidth:  termWidth,
//		isTerminal: p.isTerminal(p.output),
//		isColored:  false,
//	}

	ws.writeString(buf, p.layout.prefix)

	switch {
	case pctSigDigits >= 9950:           // 99.5% < pctSigDigits >  100% => "100%"
		ws.writeString(buf, "100")
	case pctSigDigits >= 995:            // 9.95% < pctSigDigits > 99.4% => " 10%" - " 99%"
		val := (pctSigDigits + 50) / 100 // round to the nearest 1% (995 -> 10; 9949 -> 99)
		ws.writeRune(buf, ' ')
		ws.writeRune(buf, rune('0' + (val / 10)))
		ws.writeRune(buf, rune('0' + (val % 10)))
	default:                             // 0.00% < pctSigDigits > 9.94% => "0.0%" - "9.9%"
		val := (pctSigDigits +  5) /  10 // round to the nearest 0.1% (994 -> 9.9; 0 -> 0.0)
		ws.writeRune(buf, rune('0' + (val / 10)))
		ws.writeRune(buf, '.')
		ws.writeRune(buf, rune('0' + (val % 10)))
	}

	ws.writeString(buf, p.layout.suffix)
	if truncated { ws.writeRune(buf, '…') }
	ws.writeString(buf, status)

	for p.isTerminal(p.output) && ws.visCols < ws.cols { // fill remaining bar space with clean gradient padding
		var factor int
		if ws.denom > 0 { factor = (ws.visCols * 1000) / ws.denom }

		bgR := p.theme.startBgR + (p.theme.deltaBgR * factor) / 1000
		bgG := p.theme.startBgG + (p.theme.deltaBgG * factor) / 1000
		bgB := p.theme.startBgB + (p.theme.deltaBgB * factor) / 1000

		*buf = append(*buf, prepColorSeq...)
		*buf = appendIntIdxInline(*buf, bgR)
		*buf = append(*buf, ';')
		*buf = appendIntIdxInline(*buf, bgG)
		*buf = append(*buf, ';')
		*buf = appendIntIdxInline(*buf, bgB)
		*buf = append(*buf, 'm', ' ')
		ws.visCols++
		ws.isColored = true
	}

	if p.isTerminal(p.output) && ws.isColored { *buf = append(*buf, resetAttr...) } // reset all attributes to defaults

	*buf = append(*buf, p.layout.lineTerminator...)

	_, err := p.output.Write(*buf)

	if err == nil { storeLastFrameHook(p, buf) }

	return err
}

func (ws *writeState) writeRune(buf *[]byte, r rune) {
	rWidth := 1
	if r > 0x1100 && isWideRune(r) { rWidth = 2 }

	if ws.isTerminal && ws.termWidth > 0 && ws.visCols < ws.cols {
		var factor int
		if ws.denom > 0 { factor = (ws.visCols * 1000) / ws.denom }

		bgR  := ws.theme.startBgR + (ws.theme.deltaBgR * factor) / 1000
		bgG  := ws.theme.startBgG + (ws.theme.deltaBgG * factor) / 1000
		bgB  := ws.theme.startBgB + (ws.theme.deltaBgB * factor) / 1000

		fgR  :=          startFgR + (ws.theme.deltaFgR * factor) / 1000
		fgG  :=          startFgG + (ws.theme.deltaFgG * factor) / 1000
		fgB  :=          startFgB + (ws.theme.deltaFgB * factor) / 1000

		*buf = append(*buf, setFgColor...)
		*buf = appendIntIdxInline(*buf, fgR)
		*buf = append(*buf, ';')
		*buf = appendIntIdxInline(*buf, fgG)
		*buf = append(*buf, ';')
		*buf = appendIntIdxInline(*buf, fgB)
		*buf = append(*buf, prepSetBgColor...)
		*buf = appendIntIdxInline(*buf, bgR)
		*buf = append(*buf, ';')
		*buf = appendIntIdxInline(*buf, bgG)
		*buf = append(*buf, ';')
		*buf = appendIntIdxInline(*buf, bgB)
		*buf = append(*buf, 'm')

		ws.isColored = true
	} else if ws.visCols >= ws.cols && ws.isColored {
		*buf = append(*buf, resetAttr...)
		ws.isColored = false
	}

	if r < 0x80 {
		*buf = append(*buf, byte(r & 0x7F))
	} else {
		*buf = appendRune(*buf, r)
	}

	ws.visCols += rWidth
}

func (ws *writeState) writeString(buf *[]byte, str string) {
	for _, r := range str { ws.writeRune(buf, r) }
}
