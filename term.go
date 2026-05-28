package progress

import (
	"io"
	"os"
	"testing"

	"golang.org/x/term"
)

// WithIsTerminalFunc allows callers to override internal terminal detection, e.g.:
//
//   progress.New(ctx, 100, os.Stderr, progress.WithIsTerminalFunc(func(any) bool { return true }))
func WithIsTerminalFunc(f func(any) bool) Option { return func(p *Progress) { p.isTerminal = f } }

// prepareTerminal sets the line terminator character and ANSI escape sequences to
// be used when p.output (nominally os.Stderr) has not been piped or redirected.
func (p *Progress) prepareTerminal() {
	layout := p.tracker.baseLayout()
	if p.isTerminal(p.output) {
		layout.colorBlockFactor = colorBlockFactor   // provision adequate capacity for rendering a 24-bit color block for every terminal column
		layout.clearSeq         = ansiClearSeq       // move cursor to beginning of line; freeze screen rendering for this atomic update block
		layout.doneSeq          = ansiDoneSeq        // restore all attributes to defaults; restore cursor
		layout.lineTerminator   = ansiLineTerminator // erase line; restore all attributes to defaults; flush atomic update block
	}
	p.layout = layout
}

// handleResize records the new terminal width to be respected by subsequent render cycles.
func (p *Progress) handleResize(buf []byte) []byte {
	termWidth := p.resizeHandler()

	for { // atomically update termWidth while preserving concurrent percentage or status changes
		oldState := p.state.Load()
		newState := (oldState & 0xFFFF) | (uint32(termWidth & 0xFFFF) << 16) // pack p.termWidth into upper 16 bits, retaining pctSigDigits in lower 16 bits
		if p.state.CompareAndSwap(oldState, newState) {
			if increasedBufCap := p.layout.bufCap(termWidth); cap(buf) < increasedBufCap {
				buf = make([]byte, 0, increasedBufCap) // grow the buffer to accommodate the increased terminal width
			}
			p.sync(buf)
			break
		}
	}
	return buf
}

// getResizedTermWidth returns the current terminal width, enforcing
// p.layout.staticWidth as the minimum layout threshold.
func (p *Progress) getResizedTermWidth() int {
	width := p.layout.staticWidth // assume a human manually resized the terminal, so support terminal widths as narrow as p.layout.staticWidth
	fd    := getFD(p.output)
	if fd < 0 { return width }
	if w, _, err := term.GetSize(fd); err == nil && w > 0 {
		width = max(width, w)
	}
	return width
}

// getTermWidth determines the width of the terminal
// window, which is used to format status messages.
func getTermWidth(w io.Writer) int {
	fd := getFD(w)
	if fd < 0 { return minWidth }
	if width, _, err := term.GetSize(fd); err == nil {
			return max(minWidth, width)
	}
	return minWidth
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

type resizeHandler func() int

func withResizeHandler(rh resizeHandler) Option { return func(p *Progress) { p.resizeHandler = rh } }
