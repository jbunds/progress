package progress

import (
	"strconv"
	"sync/atomic"
)

// tracker for cases where status strings are a proper fraction of the completed versus total work
type fractionTracker struct {
	current atomic.Uint64 // numerator
	total   string        // denominator (static)
}

func (f *fractionTracker) store(n float64, _ string) { f.current.Store(uint64(n)) } // stores  the numerator
func (f *fractionTracker) load()  any                { return f.current.Load()    } // returns the numerator
func (f *fractionTracker) value(v any)      string   {
	if n, ok := v.(uint64); ok {
		return strconv.FormatUint(n, 10) + "/" + f.total
	}
	return "0/" + f.total
}
