package progress

import (
	"sync/atomic"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/golang-lru/v2"
)

// standard tracker for mostly unique status strings.
type standardTracker struct {
	lo     layout
	status atomic.Value
	cache  *lru.Cache[string, string] // fixed-size, thread-safe LRU container
}

func (s *standardTracker) init() {
	s.lo = defaultLayout()
	s.status.Store("")
	c, err := lru.New[string, string](1024)
	if err != nil { panic(err) }
	s.cache = c
}

func (s *standardTracker) baseLayout() layout { return s.lo }

func (s *standardTracker) load() string {
	if val := s.status.Load(); val != nil {
		if str, ok := val.(string); ok { return str }
	}
	return ""
}

func (s *standardTracker) appendStatus(buf []byte) []byte {
	return append(buf, s.load()...)
}

func (s *standardTracker) store(_ uint64, status string) {
	if cachedVal, ok := s.cache.Get(status); ok {
		s.status.Store(cachedVal)
		return
	}
	s.cache.Add(status, status)
	s.status.Store(status)
}

func (s *standardTracker) Equal(other *standardTracker) bool { // workaround cmp's draconian strictures
	if s == nil || other == nil { return s == other }
	return cmp.Equal(s.load(), other.load()) &&
	       cmp.Equal(s.lo,     other.lo, cmp.AllowUnexported(layout{}))
}
