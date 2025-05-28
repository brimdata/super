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

type uint_ struct {
	mu   sync.Mutex
	meta *csup.Uint
	count
	vals  []uint64
	nulls *nulls
}

func newUint(cctx *csup.Context, meta *csup.Uint, nulls *nulls) *uint_ {
	return &uint_{
		meta:  meta,
		nulls: nulls,
		count: count{meta.Len(cctx), nulls.count()},
	}
}

func (*uint_) unmarshal(*csup.Context, field.Projection) {}

func (u *uint_) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, u.length())
	}
	vals, nulls := u.load(loader)
	return vector.NewUint(u.meta.Typ, vals, nulls)
}

func (u *uint_) load(loader *loader) ([]uint64, bitvec.Bits) {
	nulls := u.nulls.get(loader)
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.vals != nil {
		return u.vals, nulls
	}
	bytes := make([]byte, u.meta.Location.MemLength)
	if err := u.meta.Location.Read(loader.r, bytes); err != nil {
		panic(err)
	}
	vals := intcomp.UncompressUint64(byteconv.ReinterpretSlice[uint64](bytes), nil)
	vals = extendForNulls(vals, nulls, u.count)
	u.vals = vals
	return u.vals, nulls
}
