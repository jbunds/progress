package progress

import (
	"sync/atomic"

	"github.com/google/go-cmp/cmp"
)

// standard tracker for mostly unique status strings.
type standardTracker struct {
	lo  *layout
	ptr atomic.Pointer[string]
}

func (s *standardTracker) init()           { s.lo = defaultLayout() }
func (s *standardTracker) layout() *layout { return s.lo            }
func (s *standardTracker) load()       any { return s.ptr.Load()    }
func (s *standardTracker) store(_ uint64, status string) {
	if p := s.ptr.Load(); p == nil || *p != status {
		s.ptr.Store(&status)
	}
}

func (s *standardTracker) value(v any) string {
	if p, ok := v.(*string); ok && p != nil { return *p }
	return ""
}

func (s *standardTracker) Equal(other *standardTracker) bool { // workaround cmp's draconian strictures
	if s == nil || other == nil { return s == other }
	return cmp.Equal(s.load(), other.load()) &&
	       cmp.Equal(s.lo,     other.lo, cmp.AllowUnexported(layout{}))
}
