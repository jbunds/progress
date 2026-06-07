//go:build examples

// Package examples defines functions used by the example programs.
package examples

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jbunds/progress"
)

// Flags parses optional command line flags and returns []progress.Option to allow users to override
// the default progress.statusTracker and progress.theme used by the example API demonstration programs.
func Flags(fs *flag.FlagSet, args []string) ([]progress.Option) {
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "%s usage:\n\n", filepath.Base(fs.Name()))
		fs.PrintDefaults()
		_, _ = fmt.Fprintln(fs.Output())
	}
	var trackerStr, themeStr string
	var persistBar, forceTTY bool
	fs.StringVar(&trackerStr, "tracker",    "standard", "tracker")
	fs.StringVar(&themeStr,   "theme",      "sunset",   "theme")
	fs.BoolVar(&persistBar,   "persistbar", false,      "persist the progress bar on exit")
	fs.BoolVar(&forceTTY,     "forcetty",   false,      "force terminal capabilities even when piped or redirected")
	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v: falling back to defaults\n", err)
	}
	if len(fs.Args()) > 0 {
		_, _ = fmt.Fprintf(fs.Output(), "ignored arguments: %s\n", strings.Join(fs.Args(), ", "))
	}
	strategies := map[string]any{
		"unique":   progress.Unique,
		"percent":  progress.Percent,
		"fraction": progress.Fraction,
	}
	return []progress.Option{
		progress.WithTracker(strategies[strings.ToLower(trackerStr)]),
		progress.WithTheme(themeStr),
		progress.WithPersistBar(persistBar),
		func(p *progress.Progress) {
			if forceTTY {
				progress.WithIsTerminalFunc(func(any) bool { return true })(p)
			}
		},
	}
}
