package progress

// ANSI escape sequences
//
//   https://en.wikipedia.org/wiki/ANSI_escape_code#24-bit
//   https://en.wikipedia.org/wiki/ANSI_escape_code#Select_Graphic_Rendition_parameters
//
//   https://ecma-international.org/publications-and-standards/standards/ecma-48/
//   https://ecma-international.org/wp-content/uploads/ECMA-48_5th_edition_june_1991.pdf
//   https://www.iso.org/standard/22943.html
//
//   https://pkg.go.dev/github.com/charmbracelet/x/ansi
//   https://pkg.go.dev/github.com/charmbracelet/x/ansi#TextCursorEnableMode (DECTCEM)
//   https://pkg.go.dev/github.com/charmbracelet/x/ansi#ModeTextCursorEnable (DECTCEM)
//   https://pkg.go.dev/github.com/charmbracelet/x/ansi#ModeSynchronizedOutput
//   https://pkg.go.dev/github.com/charmbracelet/x/ansi#EraseLineRight (EraseLineRight)
//
//   https://vt100.net/docs/vt510-rm/DECRSTS.html
//   https://vt100.net/docs/vt510-rm/DECAWM.html
//
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
//   │ \033[48;2;        │ SGR sequence:
//   │                   │    48: set background...
//   │                   │     2: ...per incoming 24-bit RGB triplet
//   │ \033[38;2;        │ SGR sequence:
//   │                   │    38: set foreground (text) color...
//   │                   │     2: ...per incoming 24-bit RGB triplet
//   │                   │
//   │ \033[?25h         │ DECTCEM (DEC Text Cursor Enable Mode) high: show the cursor
//   │ \033[?25l         │ DECTCEM (DEC Text Cursor Enable Mode)  low: hide the cursor
//   │                   │
//   │   2026            │ synchronized updates mode designation number
//   │ [?2026h           │ synchronized output high:   activate synchronized output mode
//   │ [?2026l           │ synchronized output  low: deactivate synchronized output mode

const (
	ansiHideCursor     = "\033[?25l"                   // hide cursor
	ansiClearSeq       = "\r\033[K\033[?2026h\033[?7l" // move cursor to start of line; erase from cursor pos to end of line; activate sync'ed output mode; disable line wrapping
	ansiDoneSeq        = "\033[0m\r\033[?25h\033[?7h"  // reset all attributes to defaults; move cursor to beginning of line; restore cursor; enable line wrapping
	ansiLineTerminator = "\033[0m\033[?2026l"          // reset all attributes; deactivate synchronized output mode
	ansiResetAttrs     = "\033[0m"                     // reset all attributes to defaults
	ansiStartFgRGB     = "\033[38;2;"                  // set foreground (text) color (38) per incoming 24-bit RGB triplet (2)
	ansiStartBgRGB     = "\033[48;2;"                  // set background        color (48) per incoming 24-bit RGB triplet (2)
	ansiChainBgRGB     = ";48;2;"                      // chains background color (48) onto open foreground sequence per incoming 24-bit RGB triplet (2)
)
