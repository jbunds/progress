package progress

import (
	"io"
	"os"
	"testing"

	"golang.org/x/term"
)

// prepareTerminal sets the line terminator character and ANSI escape sequences to
// be used when p.output (nominally os.Stderr) has not been piped or redirected.
func (p *Progress) prepareTerminal() {
	if p.isTerminalFunc(p.output) {
		layout               := p.tracker.layout()
		layout.clearSeq       = "\r\033[?2026h"            // move cursor to beginning of line; freeze screen rendering for this atomic update block
		layout.doneSeq        = "\033[0m\r\033[?25h"       // restore all attributes to defaults; restore cursor
		layout.lineTerminator = "\033[K\033[0m\033[?2026l" // erase line; restore all attributes to defaults; flush atomic update block
	}
}

// handleResize records the new terminal width to be respected by subsequent render cycles.
func (p *Progress) handleResize() {
	bufPtr      := p.buf.Load()
	newWidth    := p.resizeHandler()
	bytesPerCol :=  4 // worst case
	padding     := 64 // conservative
	if p.isTerminal {
		bytesPerCol =  40 // "\033[38;2;255;255;255;48;2;255;255;255;m" == 36 + 4 bytes (worst case) per UTF-8 rune == 40
		padding     = 128 // additional capacity for layout.clearSeq and layout.lineTerminator sequences
	}
	reqCap := (bytesPerCol * int(newWidth)) + p.tracker.layout().staticWidth + padding

	if bufPtr == nil || cap(*bufPtr) < reqCap { // grow the buffer when the terminal width is increased
		newBuf := make([]byte, 0, reqCap)
		p.buf.Store(&newBuf)
	}

	for { // atomically update termWidth while preserving concurrent percentage or status changes
		oldState := p.state.Load()
		newState := (oldState & 0xFFFF) | (uint32(newWidth) << 16) // pack newWidth into upper 16 bits, retaining pctSigDigits in lower 16 bits
		if p.state.CompareAndSwap(oldState, newState) {
			p.sync()
			break
		}
	}
}

// getResizedTermWidth returns the current terminal width, enforcing
// p.tracker.layout().staticWidth as the minimum layout threshold.
func (p *Progress) getResizedTermWidth() uint16 {
	width := p.tracker.layout().staticWidth // assume a human manually resized the terminal, so support terminal widths as narrow as p.tracker.layout().staticWidth
	fd    := getFD(p.output)
	if fd < 0 { return uint16(width & 0xFFFF) }
	if w, _, err := term.GetSize(fd); err == nil && w > 0 {
		width = max(width, w)
	}
	p.termWidth = uint16(width & 0xFFFF)
	return p.termWidth
}

// getTermWidth determines the width of the terminal
// window, which is used to format status messages.
func getTermWidth(w io.Writer) uint16 {
	fd := getFD(w)
	if fd < 0 { return minWidth }
	if width, _, err := term.GetSize(fd); err == nil {
			return uint16(max(minWidth, width) & 0xFFFF)
	}
	return uint16(minWidth)
}

// isTerminal determines if the specified writer is connected to a terminal.
func isTerminal(v any) bool {
	if testing.Testing()                     ||
	   os.Getenv("GITHUB_ACTIONS") == "true" || // https://docs.github.com/actions/reference/workflows-and-actions/variables
	   os.Getenv("CI"            ) == "true" { return false }
	fd := getFD(v)
	if fd < 0 { return false }
	return term.IsTerminal(fd)
}

// getFD returns the file descriptor of the provided argument.
func getFD(w any) int {
	if f, ok := w.(interface{ Fd() uintptr }); ok { return int(f.Fd()) }
	return -1
}

// helpers for synchronous, deterministic tests

type resizeHandler func() uint16

func withResizeHandler(rh resizeHandler) Option { return func(p *Progress) { p.resizeHandler = rh } }
