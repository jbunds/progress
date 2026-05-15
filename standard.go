package progress

import (
	"sync/atomic"

	"github.com/google/go-cmp/cmp"
)

// standard tracker for mostly unique status strings.
type standardTracker struct {
	lo  layout
	val atomic.Value
}

func (s *standardTracker) init() {
	s.val.Store("")
	s.lo = defaultLayout()
}

func (s *standardTracker) baseLayout()           layout  { return s.lo            }
func (s *standardTracker) load()                    any  { return s.val.Load()    }
func (s *standardTracker) store(_ uint64, status string) { s.val.Store(status)    }

func (s *standardTracker) value(v any) string {
	if str, ok := v.(string); ok { return str }
	return ""
}

func (s *standardTracker) Equal(other *standardTracker) bool { // workaround cmp's draconian strictures
	if s == nil || other == nil { return s == other }
	return cmp.Equal(s.val.Load(), other.val.Load()) &&
	       cmp.Equal(s.lo,         other.lo, cmp.AllowUnexported(layout{}))
}
