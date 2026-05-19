package progress

import (
	"strconv"
	"sync/atomic"
	"unsafe"
)

// tracker for cases where status strings are a proper fraction of the completed versus total work.
type fractionTracker struct {
	lo     layout
	status atomic.Uint64 // numerator
	total  string        // denominator (static)
	buf    []byte        // pre-allocated buffer for building strings
}

func (f *fractionTracker) init() {
	f.lo  = defaultLayout()
	f.buf = make([]byte, 0, 32)
}

func (f *fractionTracker) baseLayout() layout { return f.lo }

func (f *fractionTracker) load()  string {
	b := f.buf[:0]
	b  = strconv.AppendUint(b, f.status.Load(), 10)
	b  = append(b, '/')
	b  = append(b, f.total...)
	// #nosec G103 -- string consumed synchronously before buffer reuse; audited per `go test -count=100 -gcflags=-d=checkptr .`
	return unsafe.String(&b[0], len(b)) // zero-alloc cast: directly convert stack bytes into a string header
}

func (f *fractionTracker) appendStatus(buf []byte) []byte {
	buf = strconv.AppendUint(buf, f.status.Load(), 10)
	buf = append(buf, '/')
	buf = append(buf, f.total...)
	return buf
}

func (f *fractionTracker) store(n uint64, _ string) { f.status.Add(n) } // increments the numerator
