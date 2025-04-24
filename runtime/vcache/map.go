package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type map_ struct {
	mu   sync.Mutex
	meta *csup.Map
	count
	offs   []uint32
	keys   shadow
	values shadow
	nulls  *nulls
}

func newMap(cctx *csup.Context, meta *csup.Map, nulls *nulls) *map_ {
	return &map_{
		meta:  meta,
		nulls: nulls,
		count: count{meta.Len(cctx), nulls.count()},
	}
}

func (m *map_) unmarshal(cctx *csup.Context, projection field.Projection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.keys == nil {
		m.keys = newShadow(cctx, m.meta.Keys, nil)
		m.values = newShadow(cctx, m.meta.Values, nil)
	}
	m.keys.unmarshal(cctx, projection)
	m.values.unmarshal(cctx, projection)
}

func (m *map_) project(loader *loader, projection field.Projection) vector.Any {
	keys := m.keys.project(loader, nil)
	vals := m.values.project(loader, nil)
	typ := loader.sctx.LookupTypeMap(keys.Type(), vals.Type())
	offs, nulls := m.load(loader)
	return vector.NewMap(typ, offs, keys, vals, nulls)
}

func (m *map_) load(loader *loader) ([]uint32, bitvec.Bits) {
	nulls := m.nulls.get(loader)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.offs == nil {
		offs, err := loadOffsets(loader.r, m.meta.Lengths, m.count, nulls)
		if err != nil {
			panic(err)
		}
		m.offs = offs
	}
	return m.offs, nulls
}

type mapLoader struct {
	loader *loader
	shadow *map_
}

var _ vector.Uint32Loader = (*mapLoader)(nil)

func (m *mapLoader) Load() ([]uint32, bitvec.Bits) {
	return m.shadow.load(m.loader)
}
