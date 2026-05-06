package progress

import "sync/atomic"

// standard tracker for mostly unique status strings
type standardTracker struct { ptr atomic.Pointer[string] }

func (s *standardTracker) store(_ uint64, status string) {
	if p := s.ptr.Load(); p != nil && *p == status { return }
	s.ptr.Store(&status)
}

func (s *standardTracker) load() any { return s.ptr.Load() }

func (s *standardTracker) value(v any) string {
	if p, ok := v.(*string); ok && p != nil { return *p }
	return ""
}
