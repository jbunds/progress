package progress

// (no-op) tracker for cases where status strings are not needed.
type percentTracker struct { lo layout }

// TODO(jeff): ensure redundant redraws are skipped when new shares of scale
//             are added to *Progress.current (resulting in a subsequent update
//             to *Progress.state) per Report calls, but the very low-precision
//             percentage rendered to the terminal does not change

func (p *percentTracker) init() {
	p.lo = defaultLayout()
	p.lo.suffix      = "%)"
	p.lo.finalStatus = ""
	p.lo.staticWidth = len(prefix) + pctFieldLen + len(p.lo.suffix)
}

func (p *percentTracker) baseLayout()      layout  { return p.lo }
func (p *percentTracker) load()  any               { return nil  }
func (p *percentTracker) store(_ uint64, _ string) {             }
func (p *percentTracker) value(_ any)      string  { return ""   }
