package progress

import (
	"sync/atomic"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/golang-lru/v2"
)

// standard tracker for mostly unique status strings.
type standardTracker struct {
	lo     layout
	status atomic.Pointer[string]
	cache  *lru.Cache[string, *string] // fixed-size, thread-safe LRU container
}

func (s *standardTracker) init() {
	s.lo = defaultLayout()
	s.status.Store(new(""))
	c, err := lru.New[string, *string](1024)
	if err != nil { panic(err) }
	s.cache = c
}

func (s *standardTracker) baseLayout() layout { return s.lo }

func (s *standardTracker) load() string {
	if strPtr := s.status.Load(); strPtr != nil { return *strPtr }
	return ""
}

func (s *standardTracker) appendStatus(buf []byte) []byte {
	if strPtr := s.status.Load(); strPtr != nil { return append(buf, *strPtr...) }
	return buf
}

func (s *standardTracker) store(_ uint64, status string) {
	if ptr, ok := s.cache.Get(status); ok {
		s.status.Store(ptr)
		return
	}
	ptr := &status
	s.cache.Add(status, ptr)
	s.status.Store(ptr)
}

func (s *standardTracker) Equal(other *standardTracker) bool { // workaround cmp's draconian strictures
	if s == nil || other == nil { return s == other }
	return cmp.Equal(s.status.Load(), other.status.Load()) &&
	       cmp.Equal(s.lo,            other.lo, cmp.AllowUnexported(layout{}))
}
