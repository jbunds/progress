package progress

// (no-op) tracker for cases where status strings are not needed
type percentTracker struct {}

func (p *percentTracker) store(_ float64, _ string) {}
func (p *percentTracker) load () any                { return "" }
func (p *percentTracker) value(_ any) string        { return "" }
