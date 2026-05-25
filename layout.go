package progress

// layout encapsulates the terminal-specific rendering layout configuration.
type layout struct {
	staticWidth    int    // the static width reserved for the prefix prepended to each status message, e.g., "processing (7.4%): "
	prefix         string // prepended to each progress status line rendered to the terminal
	suffix         string // appended to each status percentage calculation rendered to the terminal, e.g., "%): " or "%)"
	clearSeq       string // ANSI escape sequence used to clear the current terminal line
	doneSeq        string // ANSI escape sequence used to restore the terminal cursor
	lineTerminator string // output line terminator: "" when *Progress.output (nominally os.Stderr) is a terminal; "\n" otherwise
	finalStatus    string // status message to display upon completion (e.g., "done")
}

func defaultLayout() layout {
	layout := layout{
		prefix:         prefix,
		suffix:         defaultSuffix,
		clearSeq:       "",
		doneSeq:        "\n",
		lineTerminator: "\n",
		finalStatus:    "done",
	}
	layout.staticWidth = len(layout.prefix) + pctFieldLen + len(layout.suffix)
	return layout
}

func (l layout) bufCap(termWidth int) int {
	return (colorBlockMultiplier * termWidth) +
	       (utf8TruncMultiplier  * termWidth) +
	       len(l.prefix                     ) +
	       len(l.suffix                     ) +
	       len(l.clearSeq                   ) +
	       len(l.lineTerminator             )
}
