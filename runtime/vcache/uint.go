package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/ronanh/intcomp"
)

type uint_ struct {
	mu   sync.Mutex
	meta *csup.Uint
	len  uint32
	vals []uint64
}

func newUint(cctx *csup.Context, meta *csup.Uint) *uint_ {
	return &uint_{meta: meta, len: meta.Len(cctx)}
}

func (u *uint_) length() uint32 {
	return u.len
}

func (*uint_) unmarshal(*csup.Context, field.Projection) {}

func (u *uint_) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, u.length())
	}
	return vector.NewUint(u.meta.Typ, u.load(loader))
}

func (u *uint_) load(loader *loader) []uint64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.vals != nil {
		return u.vals
	}
	bytes := make([]byte, u.meta.Location.MemLength)
	if err := u.meta.Location.Read(loader.r, bytes); err != nil {
		panic(err)
	}
	u.vals = intcomp.UncompressUint64(byteconv.ReinterpretSlice[uint64](bytes), nil)
	return u.vals
}
