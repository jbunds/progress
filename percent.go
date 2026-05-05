package progress

// (no-op) tracker for cases where status strings are not needed
type percentTracker struct {}

// TODO(jeff): ensure redundant redraws are skipped when new shares of scale
//             are added to *Progress.current (resulting in a subsequent update
//             to *Progress.state) per Report calls, but the very low-precision
//             percentage rendered to the terminal does not change

func (p *percentTracker) store(_ float64, _ string) {}
func (p *percentTracker) load()  any                { return nil }
func (p *percentTracker) value(_ any)       string  { return ""  }
