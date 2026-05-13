package progress

const (
	startBgR,  startBgG,  startBgB =  10,  25,  12 //   0% progress theme: dark green background, white foreground
	startFgR,  startFgG,  startFgB = 255, 255, 255
	  endBgR,    endBgG,    endBgB =  40, 210,  85 // 100% progress theme: bright green background, darker grey foreground
	  endFgR,    endFgG,    endFgB =  20,  30,  20
	deltaBgR =   endBgR - startBgR                 // precomputed delta slope:  30
	deltaBgG =   endBgG - startBgG                 // precomputed delta slope: 185
	deltaBgB =   endBgB - startBgB                 // precomputed delta slope:  73
	deltaFgR = startFgR -   endFgR                 // precomputed delta slope: 235
	deltaFgG = startFgG -   endFgG                 // precomputed delta slope: 225
	deltaFgB = startFgB -   endFgB                 // precomputed delta slope: 235
)

// state tracks loop variants across drawing steps without triggering heap escapes
type writeState struct {
	buf        []byte
	cols       uint32
	visCols    uint32
	termWidth  uint32
	denom      uint32
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
	if currentVal != nil { p.lastStatusVal.Store(currentVal) }
}

// draw formats and renders the current progress status to the terminal,
// truncating text as needed to fit within the terminal width.
func (p *Progress) draw(state uint32, val any) {
	maxLen    := max(int(state >> 16) - p.tracker.layout().staticWidth, 0)
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

	layout := p.tracker.layout()
	buf    := (*bufPtr)[:0]
	buf     = append(buf, layout.clearSeq...)

	// cache loop-invariant bar gradient values
	//
	// global gradient: maps the color spectrum across the full terminal width.
	//                  the color of any character depends strictly on its absolute screen column.
	tWidth := uint32(p.termWidth)
	var denom uint32
	if tWidth > 1 { denom = tWidth - 1 }

	state := writeState{
		buf:       buf,
		visCols:   0,
		cols:      (tWidth * uint32(pctSigDigits)) / 10000,
		termWidth: tWidth,
		denom:     denom,
		isTerm:    p.isTerminal,
		isColored: false,
	}

//	// cache loop-invariant bar gradient values
//	//
//	// dynamic bar gradient: stretches the full color spectrum to fit the active bar width.
//	//                       the color gradient transitions completely from 0% to 100% inside the filled bar.
//	tWidth  := uint32(p.termWidth)
//	barCols := (tWidth * uint32(pctSigDigits)) / 10000
//	var denom uint32
//	if barCols > 1 { denom = barCols - 1 }
//
//	state := writeState{
//		buf:       buf,
//		visCols:   0,
//		cols:      barCols,
//		termWidth: tWidth,
//		denom:     denom,
//		isTerm:    p.isTerminal,
//		isColored: false,
//	}

	state.writeString(layout.prefix)

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

	state.writeString(layout.suffix)
	if truncated { state.writeRune('…') }
	state.writeString(status)

	for p.isTerminal && state.visCols < state.cols { // fill remaining bar space with clean gradient padding
		var factor uint32
		if state.denom > 0 { factor = (state.visCols * 1000) / state.denom }

		bgR := startBgR + (deltaBgR * factor) / 1000
		bgG := startBgG + (deltaBgG * factor) / 1000
		bgB := startBgB + (deltaBgB * factor) / 1000

		state.buf = append(state.buf, "\033[30;48;2;"...) // 30=black foreground (text), 48=prepare to set background color, 2=24-bit RGB triplet incoming
		state.buf = appendUintIdxInline(state.buf, bgR)
		state.buf = append(state.buf, ';')
		state.buf = appendUintIdxInline(state.buf, bgG)
		state.buf = append(state.buf, ';')
		state.buf = appendUintIdxInline(state.buf, bgB)
		state.buf = append(state.buf, 'm', ' ')
		state.visCols++
		state.isColored = true
	}

	if p.isTerminal && state.isColored { state.buf = append(state.buf, "\033[0m"...) } // reset all attributes to defaults

	state.buf = append(state.buf, layout.lineTerminator...)

	_, err := p.output.Write(state.buf)

	return err
}

func (s *writeState) writeRune(r rune) {
	var rWidth uint32 = 1
	if r > 0x1100 && isWideRune(r) { rWidth = 2 }

	if s.isTerm && s.termWidth > 0 && s.visCols < s.cols {
		var factor uint32
		if s.denom > 0 { factor = (s.visCols * 1000) / s.denom }

		bgR  := startBgR + (deltaBgR * factor) / 1000
		bgG  := startBgG + (deltaBgG * factor) / 1000
		bgB  := startBgB + (deltaBgB * factor) / 1000

		fgR  := startFgR - (deltaFgR * factor) / 1000
		fgG  := startFgG - (deltaFgG * factor) / 1000
		fgB  := startFgB - (deltaFgB * factor) / 1000

		s.buf = append(s.buf, "\033[38;2;"...) // 38=set foreground (text) color, 2=24-bit RGB triplet incoming
		s.buf = appendUintIdxInline(s.buf, fgR)
		s.buf = append(s.buf, ';')
		s.buf = appendUintIdxInline(s.buf, fgG)
		s.buf = append(s.buf, ';')
		s.buf = appendUintIdxInline(s.buf, fgB)
		s.buf = append(s.buf, ";48;2;"...) // 48=prepare to set background color, 2=24-bit RGB triplet incoming
		s.buf = appendUintIdxInline(s.buf, bgR)
		s.buf = append(s.buf, ';')
		s.buf = appendUintIdxInline(s.buf, bgG)
		s.buf = append(s.buf, ';')
		s.buf = appendUintIdxInline(s.buf, bgB)
		s.buf = append(s.buf, 'm')
		s.isColored = true
	} else if s.visCols >= s.cols && s.isColored {
		s.buf       = append(s.buf, "\033[0m"...) // reset all attributes to defaults
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
