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
	ansiHideCursor     = "\033[?25l"                // hide cursor
	ansiClearSeq       = "\r\033[?2026h"            // move cursor to beginning of line; activate synchronized output mode
	ansiDoneSeq        = "\033[0m\r\033[?25h"       // reset all attributes to defaults; move cursor to beginning of line; restore cursor
	ansiLineTerminator = "\033[K\033[0m\033[?2026l" // erase from cursor position to end of line; reset all attributes; deactivate synchronized output mode
	ansiPrepColorSeq   = "\033[30;48;2;"            // set foreground (text) color to black (30); set background color (48) per incoming 24-bit RGB triplet (2)
	ansiSetFgColor     = "\033[38;2;"               // set foreground (text) color (38) per incoming 24-bit RGB triplet (2)
	ansiPrepSetBgColor = ";48;2;"                   // prepare to set background color (48) per incoming 24-bit RGB triplet
	ansiResetAttr      = "\033[0m"                  // reset all attributes to defaults
)
