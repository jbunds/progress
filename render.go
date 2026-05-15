package progress

// ANSI escape sequences
//
//   https://en.wikipedia.org/wiki/ANSI_escape_code#24-bit
//   https://en.wikipedia.org/wiki/ANSI_escape_code#Select_Graphic_Rendition_parameters
//   https://ecma-international.org/publications-and-standards/standards/ecma-48/
//   https://jakob-bagterp.github.io/colorist-for-python/ansi-escape-codes/rgb-colors/
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
	buf        []byte
	cols       int
	visCols    int
	termWidth  int
	denom      int
	theme      *theme
	isTerm     bool
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
}

// draw formats and renders the current progress status to the terminal,
// truncating text as needed to fit within the terminal width.
func (p *Progress) draw(state uint32, val any) {
	maxLen    := max(int(state >> 16) - p.layout.staticWidth, 0)
	status    := ""
	truncated := false

	if maxLen > 0 {
		status, truncated = truncateFromLeft(p.tracker.value(val), maxLen) // truncate from left to show most relevant portion (e.g., file basename)
	}

	err := p.writeStatus(uint16(state & 0xFFFF), status, truncated)

	if p.drawNotify != nil && err == nil {
		select {
		case p.drawNotify <- struct{}{}: // ensures synchronous, deterministic tests by signaling the completion of a draw cycle
		default:
		}
	}
}

// writeStatus writes the progress status to to p.output (nominally os.Stderr)
// using the shared internal buffer to ensure an atomic system call.
func (p *Progress) writeStatus(pctSigDigits uint16, status string, truncated bool) error {
	bufPtr := p.buf.Load()
	if bufPtr == nil { return nil }

	buf := (*bufPtr)[:0]
	buf  = append(buf, p.layout.clearSeq...)

	stateWord := p.state.Load()
	termWidth := int(stateWord >> 16)

	// cache loop-invariant bar gradient values
	//
	// global gradient: maps the color spectrum across the full terminal width.
	//                  the color of any character depends strictly on its absolute screen column.
	var denom int
	if termWidth > 1 { denom = termWidth - 1 }

	state := writeState{
		buf:       buf,
		visCols:   0,
		cols:      (termWidth * int(pctSigDigits)) / 10000,
		termWidth: termWidth,
		denom:     denom,
		theme:     p.theme,
		isTerm:    p.isTerminal,
		isColored: false,
	}

//	// cache loop-invariant bar gradient values
//	//
//	// dynamic bar gradient: stretches the full color spectrum to fit the active bar width.
//	//                       the color gradient transitions completely from 0% to 100% inside the filled bar.
//	barCols := (termWidth * uint32(pctSigDigits)) / 10000
//	var denom uint32
//	if barCols > 1 { denom = barCols - 1 }
//
//	state := writeState{
//		buf:       buf,
//		visCols:   0,
//		cols:      barCols,
//		termWidth: termWidth,
//		denom:     denom,
//		theme:     p.theme,
//		isTerm:    p.isTerminal,
//		isColored: false,
//	}

	state.writeString(p.layout.prefix)

	switch {
	case pctSigDigits >= 9950:           // 99.5% < pctSigDigits >  100% => "100%"
		state.writeString("100")
	case pctSigDigits >= 995:            // 9.95% < pctSigDigits > 99.4% => " 10%" - " 99%"
		val := (pctSigDigits + 50) / 100 // round to the nearest 1% (995 -> 10; 9949 -> 99)
		state.writeRune(' ')
		state.writeRune(rune('0' + (val / 10)))
		state.writeRune(rune('0' + (val % 10)))
	default:                             // 0.00% < pctSigDigits > 9.94% => "0.0%" - "9.9%"
		val := (pctSigDigits +  5) /  10 // round to the nearest 0.1% (994 -> 9.9; 0 -> 0.0)
		state.writeRune(rune('0' + (val / 10)))
		state.writeRune('.')
		state.writeRune(rune('0' + (val % 10)))
	}

	state.writeString(p.layout.suffix)
	if truncated { state.writeRune('…') }
	state.writeString(status)

	for p.isTerminal && state.visCols < state.cols { // fill remaining bar space with clean gradient padding
		var factor int
		if state.denom > 0 { factor = (state.visCols * 1000) / state.denom }

		bgR := state.theme.startBgR + (state.theme.deltaBgR * factor) / 1000
		bgG := state.theme.startBgG + (state.theme.deltaBgG * factor) / 1000
		bgB := state.theme.startBgB + (state.theme.deltaBgB * factor) / 1000

		state.buf = append(state.buf, prepColorSeq...)
		state.buf = appendIntIdxInline(state.buf, bgR)
		state.buf = append(state.buf, ';')
		state.buf = appendIntIdxInline(state.buf, bgG)
		state.buf = append(state.buf, ';')
		state.buf = appendIntIdxInline(state.buf, bgB)
		state.buf = append(state.buf, 'm', ' ')
		state.visCols++
		state.isColored = true
	}

	if p.isTerminal && state.isColored { state.buf = append(state.buf, resetAttr...) } // reset all attributes to defaults

	state.buf = append(state.buf, p.layout.lineTerminator...)

	_, err := p.output.Write(state.buf)

	return err
}

func (s *writeState) writeRune(r rune) {
	rWidth := 1
	if r > 0x1100 && isWideRune(r) { rWidth = 2 }

	if s.isTerm && s.termWidth > 0 && s.visCols < s.cols {
		var factor int
		if s.denom > 0 { factor = (s.visCols * 1000) / s.denom }

		bgR  := s.theme.startBgR + (s.theme.deltaBgR * factor) / 1000
		bgG  := s.theme.startBgG + (s.theme.deltaBgG * factor) / 1000
		bgB  := s.theme.startBgB + (s.theme.deltaBgB * factor) / 1000

		fgR  :=         startFgR + (s.theme.deltaFgR * factor) / 1000
		fgG  :=         startFgG + (s.theme.deltaFgG * factor) / 1000
		fgB  :=         startFgB + (s.theme.deltaFgB * factor) / 1000

		s.buf = append(s.buf, setFgColor...)
		s.buf = appendIntIdxInline(s.buf, fgR)
		s.buf = append(s.buf, ';')
		s.buf = appendIntIdxInline(s.buf, fgG)
		s.buf = append(s.buf, ';')
		s.buf = appendIntIdxInline(s.buf, fgB)
		s.buf = append(s.buf, prepSetBgColor...)
		s.buf = appendIntIdxInline(s.buf, bgR)
		s.buf = append(s.buf, ';')
		s.buf = appendIntIdxInline(s.buf, bgG)
		s.buf = append(s.buf, ';')
		s.buf = appendIntIdxInline(s.buf, bgB)
		s.buf = append(s.buf, 'm')
		s.isColored = true
	} else if s.visCols >= s.cols && s.isColored {
		s.buf       = append(s.buf, resetAttr...)
		s.isColored = false
	}

	if r < 0x80 {
		s.buf = append(s.buf, byte(r & 0x7F))
	} else {
		s.buf = appendRune(s.buf, r)
	}

	s.visCols += rWidth
}

func (s *writeState) writeString(str string) {
	for _, r := range str { s.writeRune(r) }
}
