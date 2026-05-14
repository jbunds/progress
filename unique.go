package progress

import (
	"sync/atomic"
	"unique"

	"github.com/google/go-cmp/cmp"
)

// unique tracker for repetitive status strings.
type uniqueTracker struct {
	lo  *layout
	val atomic.Value
}

func (u *uniqueTracker) init()                         { u.lo = defaultLayout()           }
func (u *uniqueTracker) layout()              *layout  { return u.lo                      }
func (u *uniqueTracker) load()  any                    { return u.val.Load()              }
func (u *uniqueTracker) store(_ uint64, status string) { u.val.Store(unique.Make(status)) }
func (u *uniqueTracker) value(v any)           string  {
	if v == nil { return "" }
	if h, ok := v.(unique.Handle[string]); ok {
		return h.Value()
	}
	return ""
}

func (u *uniqueTracker) Equal(other *uniqueTracker) bool { // workaround cmp's draconian strictures
	if u == nil || other == nil { return u == other }
	return cmp.Equal(u.load(), other.load()) &&
	       cmp.Equal(u.lo,     other.lo, cmp.AllowUnexported(layout{}))
}
