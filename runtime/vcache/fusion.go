package vcache

import (
	"io"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type fusion struct {
	mu     sync.Mutex
	cctx   *csup.Context
	meta   *csup.Fusion
	len    uint32
	values shadow
	// the ids are loaded on-demand from the runtime
	subtypes subtypes
}

func newFusion(cctx *csup.Context, meta *csup.Fusion) *fusion {
	return &fusion{
		cctx:     cctx,
		meta:     meta,
		len:      meta.Len(cctx),
		subtypes: subtypes{segment: meta.Subtypes},
	}
}

func (f *fusion) length() uint32 {
	return f.len
}

func (f *fusion) unmarshal(cctx *csup.Context, projection field.Projection) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.values == nil {
		f.values = newShadow(cctx, f.meta.Values)
	}
	f.values.unmarshal(cctx, projection)
}

func (f *fusion) project(loader *loader, projection field.Projection) vector.Any {
	vec := f.values.project(loader, projection)
	typ := loader.sctx.LookupTypeFusion(vec.Type())
	l := &subtypesLoader{
		loader:   loader,
		cctx:     f.cctx,
		subtypes: &f.subtypes,
	}
	return vector.NewFusionWithLoader(loader.sctx, typ, l, vec)
}

type subtypes struct {
	// We load the IDs separately from the types so that a shadow
	// of ids may be shared by multipe contexts.
	segment csup.Segment
	mu      sync.Mutex
	ids     []uint32
}

func (s *subtypes) loadIDs(r io.ReaderAt) []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids == nil {
		ids, err := csup.ReadUint32s(s.segment, r)
		if err != nil {
			panic(err)
		}
		s.ids = ids
	}
	return s.ids
}

type subtypesLoader struct {
	loader   *loader
	cctx     *csup.Context
	subtypes *subtypes
}

var _ vector.TypeLoader = (*subtypesLoader)(nil)

func (s *subtypesLoader) Load() []super.Type {
	ids := s.subtypes.loadIDs(s.loader.r)
	s.cctx.LoadSubtypes()
	subtypes := make([]super.Type, 0, len(ids))
	sctx := s.loader.sctx
	for _, id := range ids {
		typeVal := s.cctx.LookupTypeVal(id)
		typ, err := sctx.LookupByValue(typeVal)
		if err != nil {
			panic(err)
		}
		subtypes = append(subtypes, typ)
	}
	return subtypes
}
