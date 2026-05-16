package progress

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

// writeState encapsulates canvas boundaries, color theme, and cursor positions
// across sequential draws in a stack-allocated block to prevent heap escaping.
type writeState struct {
	buf        *[]byte
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
func (p *Progress) sync() {
	currentState := p.state.Load()
	currentVal   := p.tracker.load()
	lastState    := p.lastState.Load()
	lastVal      := p.lastStatusVal.Load()

	if currentState == lastState &&
	   currentVal   == lastVal { return } // status unchanged; skip redundant redraw

	p.draw(currentState, currentVal)

	p.lastState.Store(currentState)
	p.lastStatusVal.Store(currentVal)

	// TODO(jeff); eliminate the drawNotify field by refactoring to separate the concerns
	//             of state calculations and UI rendering, so the logic can be tested
	//             synchronously by passing in a state to a pure function, and, e.g.,
	//             the renderLoop can be tested as a separate, thin integration layer
	//
	//             maybe provide a mechanism to allow callers to register a callback
	//             that fires when the desired state is reached (a callback would be
	//             better than forcing callers to poll the state via p.lastFrame)
	if p.drawNotify != nil {
		select {
		case p.drawNotify <- struct{}{}: // ensures synchronous, deterministic tests by signaling the completion of a draw cycle
		default:
		}
	}
}

// draw formats and renders the current progress status to the terminal,
// truncating text as needed to fit within the terminal width.
func (p *Progress) draw(state uint32, val any) {
	maxLen    := state >> 16 - uint32(p.layout.staticWidth & 0xFFFF)
	status    := ""
	truncated := false

	if maxLen > 0 {
		status, truncated = truncateFromLeft(p.tracker.value(val), int(maxLen)) // truncate from left to show most relevant portion (e.g., file basename)
	}

	_ = p.writeStatus(state & 0xFFFF, status, truncated)
}

// lastRenderedFrame returns the last rendered frame string.
func (p *Progress) lastRenderedFrame() string {
	val := p.lastFrame.Load()
	if val == nil { return "" }
	if v, ok := val.(string); ok { return v }
	return ""
}

// writeStatus writes the progress status to to p.output (nominally os.Stderr) per an atomic system call.
func (p *Progress) writeStatus(pctSigDigits uint32, status string, truncated bool) error {
	buf := make([]byte, 0, p.layout.bufCap(int(p.state.Load() >> 16)))
	buf  = append(buf, p.layout.clearSeq...)

	termWidth := int(p.state.Load() >> 16)

	// cache loop-invariant bar gradient values
	//
	// global gradient: maps the color spectrum across the full terminal width.
	//                  the color of any character depends strictly on its absolute screen column.
	var denom int
	if termWidth > 1 { denom = termWidth - 1 }

	ws := writeState{
		buf:        &buf,
		theme:      p.theme,
		cols:       (termWidth * int(pctSigDigits)) / 10000,
		visCols:    0,
		denom:      denom,
		termWidth:  termWidth,
		isTerminal: p.isTerminal,
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
//		buf:        &buf,
//		theme:      p.theme,
//		cols:       barCols,
//		visCols:    0,
//		denom:      denom,
//		termWidth:  termWidth,
//		isTerminal: p.isTerminal,
//		isColored:  false,
//	}

	ws.writeString(p.layout.prefix)

	switch {
	case pctSigDigits >= 9950:           // 99.5% < pctSigDigits >  100% => "100%"
		ws.writeString("100")
	case pctSigDigits >= 995:            // 9.95% < pctSigDigits > 99.4% => " 10%" - " 99%"
		val := (pctSigDigits + 50) / 100 // round to the nearest 1% (995 -> 10; 9949 -> 99)
		ws.writeRune(' ')
		ws.writeRune(rune('0' + (val / 10)))
		ws.writeRune(rune('0' + (val % 10)))
	default:                             // 0.00% < pctSigDigits > 9.94% => "0.0%" - "9.9%"
		val := (pctSigDigits +  5) /  10 // round to the nearest 0.1% (994 -> 9.9; 0 -> 0.0)
		ws.writeRune(rune('0' + (val / 10)))
		ws.writeRune('.')
		ws.writeRune(rune('0' + (val % 10)))
	}

	ws.writeString(p.layout.suffix)
	if truncated { ws.writeRune('…') }
	ws.writeString(status)

	for p.isTerminal && ws.visCols < ws.cols { // fill remaining bar space with clean gradient padding
		var factor int
		if ws.denom > 0 { factor = (ws.visCols * 1000) / ws.denom }

		bgR := p.theme.startBgR + (p.theme.deltaBgR * factor) / 1000
		bgG := p.theme.startBgG + (p.theme.deltaBgG * factor) / 1000
		bgB := p.theme.startBgB + (p.theme.deltaBgB * factor) / 1000

		*ws.buf = append(*ws.buf, prepColorSeq...)
		*ws.buf = appendIntIdxInline(*ws.buf, bgR)
		*ws.buf = append(*ws.buf, ';')
		*ws.buf = appendIntIdxInline(*ws.buf, bgG)
		*ws.buf = append(*ws.buf, ';')
		*ws.buf = appendIntIdxInline(*ws.buf, bgB)
		*ws.buf = append(*ws.buf, 'm', ' ')
		ws.visCols++
		ws.isColored = true
	}

	if p.isTerminal && ws.isColored { buf = append(buf, resetAttr...) } // reset all attributes to defaults

	buf = append(buf, p.layout.lineTerminator...)

	_, err := p.output.Write(buf)
	if err == nil { p.lastFrame.Store(string(buf)) }

	return err
}

func (ws *writeState) writeRune(r rune) {
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

		*ws.buf = append(*ws.buf, setFgColor...)
		*ws.buf = appendIntIdxInline(*ws.buf, fgR)
		*ws.buf = append(*ws.buf, ';')
		*ws.buf = appendIntIdxInline(*ws.buf, fgG)
		*ws.buf = append(*ws.buf, ';')
		*ws.buf = appendIntIdxInline(*ws.buf, fgB)
		*ws.buf = append(*ws.buf, prepSetBgColor...)
		*ws.buf = appendIntIdxInline(*ws.buf, bgR)
		*ws.buf = append(*ws.buf, ';')
		*ws.buf = appendIntIdxInline(*ws.buf, bgG)
		*ws.buf = append(*ws.buf, ';')
		*ws.buf = appendIntIdxInline(*ws.buf, bgB)
		*ws.buf = append(*ws.buf, 'm')
		ws.isColored = true
	} else if ws.visCols >= ws.cols && ws.isColored {
		*ws.buf      = append(*ws.buf, resetAttr...)
		ws.isColored = false
	}

	if r < 0x80 {
		*ws.buf = append(*ws.buf, byte(r & 0x7F))
	} else {
		*ws.buf = appendRune(*ws.buf, r)
	}

	ws.visCols += rWidth
}

func (ws *writeState) writeString(str string) {
	for _, r := range str { ws.writeRune(r) }
}
