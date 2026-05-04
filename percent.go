package progress

import "sync/atomic"

// (no-op) tracker for cases where status strings are not needed
type percentTracker struct {
	current atomic.Uint64
}

func (p *percentTracker) store(n float64, _ string) { p.current.Store(uint64(n)) }
func (p *percentTracker) load () any                { return p.current.Load()    }
func (p *percentTracker) value(_ any) string        { return ""                  }
