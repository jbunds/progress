package progress

import (
	"sync/atomic"
	"unsafe"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/golang-lru/v2"
)

// stringFrame encapsulates the status string data pointer and length within an allocated object that can be atomically swapped.
type stringFrame struct {
	strData *byte
	length  int
}

// standard tracker for mostly unique status strings.
type standardTracker struct {
	lo     layout
	status atomic.Pointer[stringFrame]
	cache  *lru.Cache[string, *stringFrame] // fixed-size, thread-safe LRU container
}

func (s *standardTracker) init() {
	s.lo      = defaultLayout()
	strFrame := &stringFrame{}
	s.status.Store(strFrame)
	c, err := lru.New[string, *stringFrame](1024)
	if err != nil { panic(err) }
	s.cache = c
}

func (s *standardTracker) store(_ uint64, status string) {
	if cachedStrFrame, ok := s.cache.Get(status); ok {
		s.status.Store(cachedStrFrame)
		return
	}
	strFrame := &stringFrame{
		strData: unsafe.StringData(status), // #nosec G103 - memory safety guarded by LRU cache retaining underlying string references
		length:  len(status),
	}
	s.cache.Add(status, strFrame)
	s.status.Store(strFrame)
}

func (s *standardTracker) load() string {
	strFrame := s.status.Load()
	if strFrame == nil || strFrame.length == 0 { return "" }
	return unsafe.String(strFrame.strData, strFrame.length) // #nosec G103 - reassembles string safely using matching length properties from the frame
}

func (s *standardTracker) addTotal(_ uint64)  {             }
func (s *standardTracker) baseLayout() layout { return s.lo }

func (s *standardTracker) Equal(other *standardTracker) bool { // work around cmp's draconian strictures
	if s == nil || other == nil { return s == other }
	return cmp.Equal(s.load(), other.load()) &&
	       cmp.Equal(s.lo,     other.lo, cmp.AllowUnexported(layout{}))
}
