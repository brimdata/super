package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type bool_ struct {
	mu   sync.Mutex
	meta *csup.Bool
	bits *bitvec.Bits
}

func newBool(meta *csup.Bool) *bool_ {
	return &bool_{meta: meta}
}

func (b *bool_) length() uint32 {
	return b.meta.Count
}

func (*bool_) unmarshal(*csup.Context, field.Projection) {}

func (b *bool_) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, b.length())
	}
	return vector.NewBool(b.load(loader))
}

func (b *bool_) load(loader *loader) bitvec.Bits {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.bits != nil {
		return *b.bits
	}
	bytes := make([]byte, b.meta.Location.MemLength)
	if err := b.meta.Location.Read(loader.r, bytes); err != nil {
		panic(err)
	}
	bits := bitvec.New(byteconv.ReinterpretSlice[uint64](bytes), b.meta.Count)
	b.bits = &bits
	return bits
}
