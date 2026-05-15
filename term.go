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
	baseLayout := p.tracker.baseLayout()
	if p.isTerminalFunc(p.output) {
		baseLayout.clearSeq       = clearSeq       // move cursor to beginning of line; freeze screen rendering for this atomic update block
		baseLayout.doneSeq        = doneSeq        // restore all attributes to defaults; restore cursor
		baseLayout.lineTerminator = lineTerminator // erase line; restore all attributes to defaults; flush atomic update block
	}
	p.layout = baseLayout
}

// handleResize records the new terminal width to be respected by subsequent render cycles.
func (p *Progress) handleResize() {
	bufPtr    := p.buf.Load()
	termWidth := p.resizeHandler()

	bufCap := (23 * int(termWidth))        +
	          ( 4 * int(termWidth))        +
	          len(p.layout.prefix        ) +
	          len(p.layout.suffix        ) +
	          len(p.layout.clearSeq      ) +
	          len(p.layout.lineTerminator)

	if bufPtr == nil || cap(*bufPtr) < bufCap { // grow the buffer when the terminal width is increased
		newBuf := make([]byte, 0, bufCap)
		p.buf.Store(&newBuf)
	}

	for { // atomically update termWidth while preserving concurrent percentage or status changes
		oldState := p.state.Load()
		newState := (oldState & 0xFFFF) | (uint32(termWidth) << 16) // pack p.termWidth into upper 16 bits, retaining pctSigDigits in lower 16 bits
		if p.state.CompareAndSwap(oldState, newState) {
			p.sync()
			break
		}
	}
}

// getResizedTermWidth returns the current terminal width, enforcing
// p.layout.staticWidth as the minimum layout threshold.
func (p *Progress) getResizedTermWidth() uint16 {
	width := p.layout.staticWidth // assume a human manually resized the terminal, so support terminal widths as narrow as p.layout.staticWidth
	fd    := getFD(p.output)
	if fd < 0 { return uint16(width & 0xFFFF) }
	if w, _, err := term.GetSize(fd); err == nil && w > 0 {
		width = max(width, w)
	}
	return uint16(width & 0xFFFF)
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
