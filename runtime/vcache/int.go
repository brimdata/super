package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/ronanh/intcomp"
)

type int_ struct {
	mu   sync.Mutex
	meta *csup.Int
	len  uint32
	vals []int64
}

func newInt(cctx *csup.Context, meta *csup.Int) *int_ {
	return &int_{meta: meta, len: meta.Len(cctx)}
}

func (i *int_) length() uint32 {
	return i.len
}

func (*int_) unmarshal(*csup.Context, field.Projection) {}

func (i *int_) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, i.length())
	}
	return vector.NewInt(i.meta.Typ, i.load(loader))
}

func (i *int_) load(loader *loader) []int64 {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.vals != nil {
		return i.vals
	}
	bytes := make([]byte, i.meta.Location.MemLength)
	if err := i.meta.Location.Read(loader.r, bytes); err != nil {
		panic(err)
	}
	i.vals = intcomp.UncompressInt64(byteconv.ReinterpretSlice[uint64](bytes), nil)
	return i.vals
}
