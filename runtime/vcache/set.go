package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type set struct {
	mu     sync.Mutex
	meta   *csup.Set
	len    uint32
	offs   []uint32
	values shadow
}

func newSet(cctx *csup.Context, meta *csup.Set) *set {
	return &set{meta: meta, len: meta.Len(cctx)}
}

func (s *set) length() uint32 {
	return s.len
}

func (s *set) unmarshal(cctx *csup.Context, projection field.Projection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.values == nil {
		s.values = newShadow(cctx, s.meta.Values)
	}
	s.values.unmarshal(cctx, projection)
}

func (s *set) project(loader *loader, projection field.Projection) vector.Any {
	vec := s.values.project(loader, nil)
	typ := loader.sctx.LookupTypeSet(vec.Type())
	offs := s.load(loader)
	return vector.NewSet(typ, offs, vec)
}

func (s *set) load(loader *loader) []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.offs != nil {
		return s.offs
	}
	offs, err := csup.ReadUint32s(s.meta.Lengths, loader.r)
	if err != nil {
		panic(err)
	}
	s.offs = offs
	return offs
}
