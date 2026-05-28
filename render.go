package progress

// zero-overhead production / default no-op stubs:
// - decoupled from the production runtime path via compiler optimization.
// - overridden during test execution per init_test.go.
// - enables instrumentation hooks to be injected to ensure synchronous, deterministic tests.
var (
	syncCompleteHook   = func(*Progress        ) {}
	storeLastFrameHook = func(*Progress, []byte) {}
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
func (p *Progress) sync(buf []byte) []byte {
	defer func() { syncCompleteHook(p) }()

	currentState     := p.state.Load()
	lastState        := p.lastState.Load()
	currentStatusVal := p.tracker.load()
	lastStatusVal    := p.lastStatusVal.Load()

	// the following conditional used to skip redundant redraws can ignore the ANSI
	// sequences wrapping every column of the terminal; the combination of p.state
	// (termWidth & pctSigDigits) and Report()ed status string is sufficient

	if currentState     == lastState &&
	   currentStatusVal == lastStatusVal { return buf } // state & status unchanged; skip redundant redraw

	buf = p.draw(buf, currentState)

	p.lastState.Store(currentState)
	p.lastStatusVal.Store(currentStatusVal)

	return buf
}

// draw formats and renders the current progress status to the terminal,
// truncating text as needed to fit within the terminal width.
func (p *Progress) draw(buf []byte, state uint32) []byte {
	maxLen    := state >> 16 - uint32(p.layout.staticWidth & 0xFFFF)
	status    := ""
	truncated := false

	if maxLen > 0 {
		status, truncated = truncateFromLeft(p.tracker.load(), int(maxLen)) // truncate from left to show most relevant portion (e.g., file basename)
	}

	buf, _ = p.writeStatus(buf, state & 0xFFFF, status, truncated)

	return buf
}

// lastFrameRendered returns the last rendered frame string.
func (p *Progress) lastFrameRendered() string {
	if v := p.lastFrame.Load(); v != nil { return *v }
	return ""
}

// writeStatus writes the progress status to to p.output (nominally os.Stderr) per an atomic system call.
func (p *Progress) writeStatus(buf []byte, pctSigDigits uint32, status string, truncated bool) ([]byte, error) {
	buf = append(buf, p.layout.clearSeq...)

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

	buf = ws.writeString(buf, p.layout.prefix)

	switch {
	case pctSigDigits >= 9950:           // 99.5% < pctSigDigits >  100% => "100%"
		buf = ws.writeString(buf, "100")
	case pctSigDigits >=  995:           // 9.95% < pctSigDigits > 99.4% => " 10%" - " 99%"
		val := (pctSigDigits + 50) / 100 // round to the nearest 1% (995 -> 10; 9949 -> 99)
		buf = ws.writeRune(buf, ' ')
		buf = ws.writeRune(buf, rune('0' + (val / 10)))
		buf = ws.writeRune(buf, rune('0' + (val % 10)))
	default:                             // 0.00% < pctSigDigits > 9.94% => "0.0%" - "9.9%"
		val := (pctSigDigits +  5) /  10 // round to the nearest 0.1% (994 -> 9.9; 0 -> 0.0)
		buf = ws.writeRune(buf, rune('0' + (val / 10)))
		buf = ws.writeRune(buf, '.')
		buf = ws.writeRune(buf, rune('0' + (val % 10)))
	}

	buf = ws.writeString(buf, p.layout.suffix)
	if truncated { buf = ws.writeRune(buf, '…') }
	buf = ws.writeString(buf, status)

	for p.isTerminal(p.output) && ws.visCols < ws.cols { // fill remaining bar space with clean gradient padding
		var factor int
		if ws.denom > 0 { factor = (ws.visCols * 1000) / ws.denom }

		bgR := p.theme.startBgR + (p.theme.deltaBgR * factor) / 1000
		bgG := p.theme.startBgG + (p.theme.deltaBgG * factor) / 1000
		bgB := p.theme.startBgB + (p.theme.deltaBgB * factor) / 1000

		buf = append(buf, ansiPrepColorSeq...)
		buf = appendIntIdxInline(buf, bgR)
		buf = append(buf, ';')
		buf = appendIntIdxInline(buf, bgG)
		buf = append(buf, ';')
		buf = appendIntIdxInline(buf, bgB)
		buf = append(buf, 'm', ' ')
		ws.visCols++
		ws.isColored = true
	}

	if p.isTerminal(p.output) && ws.isColored { buf = append(buf, ansiResetAttr...) } // reset all attributes to defaults

	buf = append(buf, p.layout.lineTerminator...)

	_, err := p.output.Write(buf)

	if err == nil { storeLastFrameHook(p, buf) }

	return buf, err
}

func (ws *writeState) writeRune(buf []byte, r rune) []byte {
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

		buf = append(buf, ansiSetFgColor...)
		buf = appendIntIdxInline(buf, fgR)
		buf = append(buf, ';')
		buf = appendIntIdxInline(buf, fgG)
		buf = append(buf, ';')
		buf = appendIntIdxInline(buf, fgB)
		buf = append(buf, ansiPrepSetBgColor...)
		buf = appendIntIdxInline(buf, bgR)
		buf = append(buf, ';')
		buf = appendIntIdxInline(buf, bgG)
		buf = append(buf, ';')
		buf = appendIntIdxInline(buf, bgB)
		buf = append(buf, 'm')

		ws.isColored = true
	} else if ws.visCols >= ws.cols && ws.isColored {
		buf = append(buf, ansiResetAttr...)
		ws.isColored = false
	}

	if r < 0x80 {
		buf = append(buf, byte(r & 0x7F))
	} else {
		buf = appendRune(buf, r)
	}

	ws.visCols += rWidth

	return buf
}

func (ws *writeState) writeString(buf []byte, str string) []byte {
	for _, r := range str { buf = ws.writeRune(buf, r) }
	return buf
}
