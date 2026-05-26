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

func (s *standardTracker) store(_ uint64, status string) {
	if cachedPointer, ok := s.cache.Get(status); ok {
		s.status.Store(cachedPointer)
		return
	}
	strCopy := status // zero-alloc local copy to isolate the string header
	s.cache.Add(status, &strCopy)
	s.status.Store(&strCopy)
}

func (s *standardTracker) load() string {
	if ptr := s.status.Load(); ptr != nil { return *ptr }
	return ""
}

func (s *standardTracker) addTotal(_ uint64)  {             }
func (s *standardTracker) baseLayout() layout { return s.lo }

func (s *standardTracker) Equal(other *standardTracker) bool { // work around cmp's draconian strictures
	if s == nil || other == nil { return s == other }
	return cmp.Equal(s.load(), other.load()) &&
	       cmp.Equal(s.lo,     other.lo, cmp.AllowUnexported(layout{}))
}
