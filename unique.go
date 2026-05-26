package progress

import (
	"sync/atomic"
	"unique"

	"github.com/google/go-cmp/cmp"
//	"github.com/google/go-cmp/cmp/cmpopts"
)

// unique tracker for repetitive status strings.
type uniqueTracker struct {
	lo     layout
	status atomic.Value
}

func (u *uniqueTracker) init() {
	u.status.Store(unique.Make(""))
	u.lo = defaultLayout()
}

func (u *uniqueTracker) store(_ uint64, status string) { u.status.Store(unique.Make(status)) }

func (u *uniqueTracker) load() string {
	if status, ok := u.status.Load().(unique.Handle[string]); ok { return status.Value() }
	return ""
}

func (u *uniqueTracker) addTotal(_ uint64)  {             }
func (u *uniqueTracker) baseLayout() layout { return u.lo }

func (u *uniqueTracker) Equal(other *uniqueTracker) bool { // work around cmp's draconian strictures
	if u == nil || other == nil { return u == other }
	return cmp.Equal(u.load(), other.load()) &&
	       cmp.Equal(u.lo,     other.lo,     cmp.AllowUnexported(layout{}))
}
