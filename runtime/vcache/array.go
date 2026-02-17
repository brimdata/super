package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type array struct {
	mu     sync.Mutex
	meta   *csup.Array
	len    uint32
	offs   []uint32
	values shadow
}

func newArray(cctx *csup.Context, meta *csup.Array) *array {
	return &array{meta: meta, len: meta.Len(cctx)}
}

func (a *array) length() uint32 {
	return a.len
}

func (a *array) unmarshal(cctx *csup.Context, projection field.Projection) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.values == nil {
		a.values = newShadow(cctx, a.meta.Values)
	}
	a.values.unmarshal(cctx, projection)
}

func (a *array) project(loader *loader, projection field.Projection) vector.Any {
	vec := a.values.project(loader, nil)
	typ := loader.sctx.LookupTypeArray(vec.Type())
	offs := a.load(loader)
	return vector.NewArray(typ, offs, vec)
}

func (a *array) load(loader *loader) []uint32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.offs != nil {
		return a.offs
	}
	offs, err := csup.ReadUint32s(a.meta.Lengths, loader.r)
	if err != nil {
		panic(err)
	}
	a.offs = offs
	return offs
}
