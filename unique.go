package progress

import (
	"sync/atomic"
	"unique"
)

// unique tracker for repetitive status strings
type uniqueTracker struct {
	lo  layout
	val atomic.Value
}

func (u *uniqueTracker) init()                         { *u.layout() = defaultLayout()    }
func (u *uniqueTracker) layout()              *layout  { return &u.lo                     }
func (u *uniqueTracker) load()  any                    { return u.val.Load()              }
func (u *uniqueTracker) store(_ uint64, status string) { u.val.Store(unique.Make(status)) }
func (u *uniqueTracker) value(v any)           string  {
	if v == nil { return "" }
	if h, ok := v.(unique.Handle[string]); ok {
		return h.Value()
	}
	return ""
}
