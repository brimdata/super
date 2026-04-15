package vcache

import (
	"io"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type typevalue struct {
	// We load the IDs separately from the types so that a shadow
	// of ids may be shared by multipe contexts.
	meta *csup.TypeValue
	mu   sync.Mutex
	ids  []uint32
	len  uint32
}

func newTypeValue(cctx *csup.Context, meta *csup.TypeValue) *typevalue {
	return &typevalue{
		meta: meta,
		len:  meta.Len(cctx),
	}
}

func (t *typevalue) length() uint32 {
	return t.len
}

func (*typevalue) unmarshal(*csup.Context, field.Projection) {}

func (t *typevalue) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, t.length())
	}
	return vector.NewTypeValueWithLoader(loader.sctx, t.newLoader(loader))
}

func (t *typevalue) newLoader(loader *loader) *typesLoader {
	return &typesLoader{loader, t}
}

func (s *typevalue) loadIDs(r io.ReaderAt) []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids == nil {
		ids, err := csup.ReadUint32s(s.meta.Location, r)
		if err != nil {
			panic(err)
		}
		s.ids = ids
	}
	return s.ids
}

type typesLoader struct {
	loader *loader
	type_  *typevalue
}

var _ vector.TypesLoader = (*typesLoader)(nil)

func (s *typesLoader) Load() (*super.TypeDefs, []uint32) {
	return s.loader.cctx.LoadSubtypes(), s.type_.loadIDs(s.loader.r)

}
