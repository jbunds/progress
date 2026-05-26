package progress

import "sync"

// buf wraps a byte slice so that bufPool methods operate on pointers.
type buf []byte

// bufPool is a type-safe wrapper around sync.Pool for byte slices.
type bufPool struct { pool sync.Pool }

// newBufPool is a thin wrapper to avoid type assertions in business logic and enforce compile-time type safety.
func newBufPool(factory func() []byte) *bufPool {
	return &bufPool{
		pool: sync.Pool{
			New: func() any {
				b := buf(factory())
				return &b
			},
		},
	}
}

// get returns a byte slice reset to 0 length, retaining capacity.
func (p *bufPool) get() []byte {
	// nolint:forcetypeassert // underlying sync.Pool guarantees *buf is returned
	bPtr := p.pool.Get().(*buf)
	return (*bPtr)[:0]
}

// put returns the byte slice to the pool.
func (p *bufPool) put(x []byte) {
	b := buf(x)
	p.pool.Put(&b)
}

// initBufPool initializes a pool of reusable naked buffers with adequate capacity to accommodate
// rendering 256 columns of 24-bit color ANSI escape sequence-wrapped characters to the terminal.
func (p *Progress) initBufPool() {
	termWidth := int(p.state.Load() >> 16)
	if p.isTerminal(p.output) && termWidth < 256 { termWidth = 256 }
	p.bufPool = newBufPool(func() []byte { return make([]byte, 0, p.layout.bufCap(termWidth)) })
}
