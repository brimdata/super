package zngio

import (
	"slices"
	"sync"
	"sync/atomic"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zbuf"
)

type batch struct {
	buf  *buffer
	refs int32
	vals []super.Value
}

var _ zbuf.Batch = (*batch)(nil)

var batchPool sync.Pool

func newBatch(buf *buffer) *batch {
	b, ok := batchPool.Get().(*batch)
	if !ok {
		b = &batch{vals: make([]super.Value, 200)}
	}
	b.buf = buf
	b.refs = 1
	b.vals = b.vals[:0]
	return b
}

func (b *batch) extend() *super.Value {
	n := len(b.vals)
	b.vals = slices.Grow(b.vals, 1)[:n+1]
	return &b.vals[n]
}

// unextend undoes what extend did.
func (b *batch) unextend() {
	b.vals = b.vals[:len(b.vals)-1]
}

func (b *batch) Ref() { atomic.AddInt32(&b.refs, 1) }

func (b *batch) Unref() {
	if refs := atomic.AddInt32(&b.refs, -1); refs == 0 {
		if b.buf != nil {
			b.buf.free()
			b.buf = nil
		}
		batchPool.Put(b)
	} else if refs < 0 {
		panic("zngio: negative batch reference count")
	}
}

func (b *batch) Values() []super.Value { return b.vals }

// XXX this should be ok, but we should handle nil receiver in scope so push
// will do the right thing
func (*batch) Vars() []super.Value { return nil }
