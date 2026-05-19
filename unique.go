package progress

import (
	"sync/atomic"
	"unique"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

func (u *uniqueTracker) baseLayout() layout { return u.lo }

func (u *uniqueTracker) load() string {
	if status, ok := u.status.Load().(unique.Handle[string]); ok { return status.Value() }
	return ""
}

func (u *uniqueTracker) appendStatus(_ []byte) []byte { return nil }

func (u *uniqueTracker) store(_ uint64, status string) { u.status.Store(unique.Make(status)) }

func (u *uniqueTracker) Equal(other *uniqueTracker) bool { // workaround cmp's draconian strictures
	if u == nil || other == nil { return u == other }
	uVal, _ :=     u.status.Load().(unique.Handle[string])
	oVal, _ := other.status.Load().(unique.Handle[string])
	return cmp.Equal(uVal, oVal,     cmpopts.EquateComparable(unique.Handle[string]{})) &&
	       cmp.Equal(u.lo, other.lo, cmp.AllowUnexported(layout{}))
}
