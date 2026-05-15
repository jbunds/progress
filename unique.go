package progress

import (
	"sync/atomic"
	"unique"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// unique tracker for repetitive status strings.
type uniqueTracker struct {
	lo  layout
	val atomic.Value
}

func (u *uniqueTracker) init() {
	u.val.Store(unique.Make(""))
	u.lo = defaultLayout()
}

func (u *uniqueTracker) baseLayout()           layout  { return u.lo                      }
func (u *uniqueTracker) load()  any                    { return u.val.Load()              }
func (u *uniqueTracker) store(_ uint64, status string) { u.val.Store(unique.Make(status)) }
func (u *uniqueTracker) value(v any)           string  {
	if h, ok := v.(unique.Handle[string]); ok { return h.Value() }
	return ""
}

func (u *uniqueTracker) Equal(other *uniqueTracker) bool { // workaround cmp's draconian strictures
	if u == nil || other == nil { return u == other }
	uVal, _ :=     u.val.Load().(unique.Handle[string])
	oVal, _ := other.val.Load().(unique.Handle[string])
	return cmp.Equal(uVal, oVal,     cmpopts.EquateComparable(unique.Handle[string]{})) &&
	       cmp.Equal(u.lo, other.lo, cmp.AllowUnexported(layout{}))
}
