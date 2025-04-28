package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type array struct {
	mu   sync.Mutex
	meta *csup.Array
	count
	offs   []uint32
	values shadow
	nulls  *nulls
}

func newArray(cctx *csup.Context, meta *csup.Array, nulls *nulls) *array {
	return &array{
		meta:  meta,
		nulls: nulls,
		count: count{meta.Len(cctx), nulls.count()},
	}
}

func (a *array) unmarshal(cctx *csup.Context, projection field.Projection) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.values == nil {
		a.values = newShadow(cctx, a.meta.Values, nil)
	}
	a.values.unmarshal(cctx, projection)
}

func (a *array) project(loader *loader, projection field.Projection) vector.Any {
	vec := a.values.project(loader, nil)
	typ := loader.sctx.LookupTypeArray(vec.Type())
	offs, nulls := a.load(loader)
	return vector.NewArray(typ, offs, vec, nulls)
}

func (a *array) load(loader *loader) ([]uint32, bitvec.Bits) {
	nulls := a.nulls.get(loader)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.offs == nil {
		offs, err := loadOffsets(loader.r, a.meta.Lengths, a.count, nulls)
		if err != nil {
			panic(err)
		}
		a.offs = offs
	}
	return a.offs, nulls
}

type arrayLoader struct {
	loader *loader
	shadow *array
}

var _ vector.Uint32Loader = (*arrayLoader)(nil)

func (a *arrayLoader) Load() ([]uint32, bitvec.Bits) {
	return a.shadow.load(a.loader)
}
