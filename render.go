package progress

import "github.com/mattn/go-runewidth"

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
	curBarEnd  int // column index indicating the end of current progress bar
	curColPos  int // tracks current column position during string serialization; used to compute gradient color fractions
	termWidth  int
	isTerminal bool
	isColored  bool
}

// lastFrameRendered returns the last rendered frame string and is used only in tests.
func (p *Progress) lastFrameRendered() string {
	if v := p.lastFrame.Load(); v != nil { return *v }
	return ""
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

// writeStatus writes the progress status to to p.output (nominally os.Stderr) per an atomic system call.
func (p *Progress) writeStatus(buf []byte, pctSigDigits uint32, status string, truncated bool) ([]byte, error) {
	buf = append(buf, p.layout.clearSeq...)

	termWidth := int(p.state.Load() >> 16)

	// cache loop-invariant bar gradient values
	//
	// global gradient: maps the color spectrum across the full terminal width.
	//                  the color of any character depends strictly on its absolute screen column.
	ws := writeState{
		theme:      p.theme,
		curBarEnd:  (termWidth * int(pctSigDigits)) / 10000,
		curColPos:  0,
		termWidth:  termWidth,
		isTerminal: p.isTerminal(p.output),
		isColored:  false,
	}

//	// cache loop-invariant bar gradient values
//	//
//	// dynamic bar gradient: stretches the full color spectrum to fit the active bar width.
//	//                       the color gradient transitions completely from 0% to 100% inside the filled bar.
//	barCols := (termWidth * int(pctSigDigits)) / 10000
//
//	ws := writeState{
//		theme:      p.theme,
//		curBarEnd:  barCols,
//		curColPos:  0,
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

	for ws.isTerminal && ws.curColPos < ws.curBarEnd { // fill remaining bar space with clean gradient padding
		color := ws.theme.bgColor(float64(ws.curColPos) / float64(ws.termWidth - 1))

		buf = append(buf, ansiStartBgRGB...)
		buf = appendRGBInline(buf, color.r)
		buf = append(buf, ';')
		buf = appendRGBInline(buf, color.g)
		buf = append(buf, ';')
		buf = appendRGBInline(buf, color.b)
		buf = append(buf, 'm', ' ')
		ws.curColPos++
		ws.isColored = true
	}

	if ws.isTerminal && ws.isColored {
		buf = append(buf, ansiResetAttrs...) // reset all attributes to defaults
		ws.isColored = false
	}

	buf = append(buf, p.layout.lineTerminator...)

	_, err := p.output.Write(buf)

	if err == nil { storeLastFrameHook(p, buf) }

	return buf, err
}

func (ws *writeState) writeRune(buf []byte, r rune) []byte {
	rWidth := 1
	if runewidth.RuneWidth(r) == 2 { rWidth = 2 }

	if ws.isTerminal && ws.termWidth > 0 && ws.curColPos < ws.curBarEnd {
		bg := ws.theme.bgColor(float64(ws.curColPos) / float64(ws.termWidth - 1))
		fg := bg.fgColor()

		buf = append(buf, ansiStartFgRGB...) // write foreground color sequence (\033[38;2;R;G;Bm)
		buf = appendRGBInline(buf, fg.r)
		buf = append(buf, ';')
		buf = appendRGBInline(buf, fg.g)
		buf = append(buf, ';')
		buf = appendRGBInline(buf, fg.b)
		buf = append(buf, ansiChainBgRGB...) // write background color sequence (48;2;R;G;Bm)
		buf = appendRGBInline(buf, bg.r)
		buf = append(buf, ';')
		buf = appendRGBInline(buf, bg.g)
		buf = append(buf, ';')
		buf = appendRGBInline(buf, bg.b)
		buf = append(buf, 'm')

		ws.isColored = true
	} else if ws.curColPos >= ws.curBarEnd && ws.isColored {
		buf = append(buf, ansiResetAttrs...)
		ws.isColored = false
	}

	if r < 0x80 {
		buf = append(buf, byte(r & 0x7F))
	} else {
		buf = appendRune(buf, r)
	}

	ws.curColPos += rWidth

	return buf
}

func (ws *writeState) writeString(buf []byte, str string) []byte {
	for _, r := range str { buf = ws.writeRune(buf, r) }
	return buf
}
