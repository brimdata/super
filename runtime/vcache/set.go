package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type set struct {
	mu   sync.Mutex
	meta *csup.Set
	count
	offs   []uint32
	values shadow
	nulls  *nulls
}

func newSet(cctx *csup.Context, meta *csup.Set, nulls *nulls) *set {
	return &set{
		meta:  meta,
		nulls: nulls,
		count: count{meta.Len(cctx), nulls.count()},
	}
}

func (s *set) unmarshal(cctx *csup.Context, projection field.Projection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.values == nil {
		s.values = newShadow(cctx, s.meta.Values, nil)
	}
	s.values.unmarshal(cctx, projection)
}

func (s *set) project(loader *loader, projection field.Projection) vector.Any {
	vec := s.values.project(loader, nil)
	typ := loader.sctx.LookupTypeSet(vec.Type())
	offs, nulls := s.load(loader)
	return vector.NewSet(typ, offs, vec, nulls)
}

func (s *set) load(loader *loader) ([]uint32, bitvec.Bits) {
	nulls := s.nulls.get(loader)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.offs == nil {
		offs, err := loadOffsets(loader.r, s.meta.Lengths, s.count, nulls)
		if err != nil {
			panic(err)
		}
		s.offs = offs
	}
	return s.offs, nulls
}

type setLoader struct {
	loader *loader
	shadow *set
}

var _ vector.Uint32Loader = (*setLoader)(nil)

func (s *setLoader) Load() ([]uint32, bitvec.Bits) {
	return s.shadow.load(s.loader)
}
