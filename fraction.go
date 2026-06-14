package progress

import (
	"strconv"
	"sync/atomic"
	"unsafe"
)

// tracker for cases where status strings are a proper fraction of the completed versus total work.
type fractionTracker struct {
	lo           layout
	initialTotal uint64        // TODO(jeff): this is ugly... find a cleaner way to do this
	status       atomic.Uint64 // numerator
	total        atomic.Uint64 // denominator
	buf          []byte        // pre-allocated buffer for building strings
}

func (f *fractionTracker) init() {
	f.lo = defaultLayout()
	f.total.Store(f.initialTotal)
	f.buf = make([]byte, 0, f.lo.bufCap(minWidth))
}

func (f *fractionTracker) store(n uint64, _ string) { f.status.Add(n) } // increments the numerator

func (f *fractionTracker) load() string {
	b := f.buf[:0]
	b  = strconv.AppendUint(b, f.status.Load(), 10)
	b  = append(b, '/')
	b  = strconv.AppendUint(b, f.total.Load(), 10)
	// #nosec G103 - string consumed synchronously before buffer reuse;
	//               continuously audited per `go test -count=100 -gcflags=-d=checkptr .` via .pre-commit-config.yaml
	return unsafe.String(&b[0], len(b)) // zero-alloc cast: convert stack bytes into a string header
}

func (f *fractionTracker) addTotal(n uint64) {
	newTotal := min(f.total.Load() + n, scale)
	f.total.Store(newTotal)
	f.lo.finalStatus = strconv.FormatUint(newTotal, 10) + "/" + strconv.FormatUint(newTotal, 10)
}

func (f *fractionTracker) layout() layout       { return f.lo }
func (f *fractionTracker) setLayout(lo *layout) { f.lo = *lo  }
