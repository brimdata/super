package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/ronanh/intcomp"
)

type int_ struct {
	mu   sync.Mutex
	meta *csup.Int
	count
	vals  []int64
	nulls *nulls
}

func newInt(cctx *csup.Context, meta *csup.Int, nulls *nulls) *int_ {
	return &int_{
		meta:  meta,
		nulls: nulls,
		count: count{meta.Len(cctx), nulls.count()},
	}
}

func (*int_) unmarshal(*csup.Context, field.Projection) {}

func (i *int_) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, i.length())
	}
	vals, nulls := i.load(loader)
	return vector.NewInt(i.meta.Typ, vals, nulls)
}

func (i *int_) load(loader *loader) ([]int64, bitvec.Bits) {
	nulls := i.nulls.get(loader)
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.vals != nil {
		return i.vals, nulls
	}
	bytes := make([]byte, i.meta.Location.MemLength)
	if err := i.meta.Location.Read(loader.r, bytes); err != nil {
		panic(err)
	}
	vals := intcomp.UncompressInt64(byteconv.ReinterpretSlice[uint64](bytes), nil)
	vals = extendForNulls(vals, nulls, i.count)
	i.vals = vals
	return vals, nulls
}

type intLoader struct {
	loader *loader
	shadow *int_
}

var _ vector.IntLoader = (*intLoader)(nil)

func (i *intLoader) Load() ([]int64, bitvec.Bits) {
	return i.shadow.load(i.loader)
}
