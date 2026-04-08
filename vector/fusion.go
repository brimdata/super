package vector

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Fusion struct {
	Sctx   *super.Context
	Typ    *super.TypeFusion
	Values Any
	// The fusion subtypes are always created as typedefs, whether
	// loaded from CSUP, built from a fuse operation, built from FJSON
	// etc.  They are only materialized into the query context when
	// absoluately necessary to perform less common operations that require
	// detailed types.  When coming from CSUP, the typedefs are lazily loaded
	// and often never even read from storage.
	mu       sync.Mutex
	loader   TypesLoader
	subtypes []super.Type
}

// TypesLoader is an interface to load types as IDs and a TypeDefs table so
// they do not pollute the query type context and are converted only when
// needed.  For example, the CSUP reader reads the typedefs table only when
// there is a runtime call to do so, and the CSUP writer loads types as
// TypeDefs for merging and writing to metadata without ever needing to
// create any super.Types.
type TypesLoader interface {
	Load() (*super.TypeDefs, []uint32)
}

var _ Any = (*Fusion)(nil)

func NewFusion(sctx *super.Context, typ *super.TypeFusion, vals Any, subtypes []super.Type) *Fusion {
	return &Fusion{Sctx: sctx, Typ: typ, Values: vals, subtypes: subtypes}
}

func NewFusionWithLoader(sctx *super.Context, typ *super.TypeFusion, loader TypesLoader, vals Any) *Fusion {
	return &Fusion{Sctx: sctx, Typ: typ, loader: loader, Values: vals}
}

func (*Fusion) Kind() Kind {
	return KindFusion
}

func (f *Fusion) Type() super.Type {
	return f.Typ
}

func (f *Fusion) Len() uint32 {
	return f.Values.Len()
}

func (f *Fusion) Serialize(b *scode.Builder, slot uint32) {
	b.BeginContainer()
	f.Values.Serialize(b, slot)
	// XXX this is a slow path
	typeVal := f.Sctx.LookupTypeValue(f.Subtypes()[slot])
	b.Append(typeVal.Bytes())
	b.EndContainer()
}

// SubtypeIDs returns the typedefs table local to the fusion vector (e.g., not
// with respect to the query context).  These IDs are then mappable into
// the query context with a super.TypeDefsMapper or into a common typedefs
// table with a super.TypeDefsMerger as is done in the CSUP write path.
func (f *Fusion) SubtypeIDs() (*super.TypeDefs, []uint32) {
	if f.loader == nil {
		// If there's no loader, there must be a set of super.Types in the
		// subtypes array.  This currently only happens when building vector.Fusion
		// from the sam.
		f.loader = f.buildTypeDefs()
	}
	return f.loader.Load()
}

type loaderShim struct {
	defs *super.TypeDefs
	ids  []uint32
}

var _ TypesLoader = (*loaderShim)(nil)

func (l *loaderShim) Load() (*super.TypeDefs, []uint32) {
	return l.defs, l.ids
}

func (f *Fusion) buildTypeDefs() *loaderShim {
	defs := super.NewTypeDefs()
	ids := make([]uint32, 0, len(f.subtypes))
	for _, typ := range f.subtypes {
		// This lookup has the side effect of installing each needed typedef
		// in the defs table
		ids = append(ids, defs.LookupType(typ))
	}
	return &loaderShim{defs, ids}
}

func (f *Fusion) Subtypes() []super.Type {
	// XXX holding look during I/O
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.subtypes == nil {
		defs, ids := f.SubtypeIDs()
		subtypes := make([]super.Type, 0, f.Values.Len())
		mapper := super.NewTypeDefsMapper(f.Sctx, defs)
		for _, id := range ids {
			typ := mapper.LookupType(id)
			if typ == nil {
				// Panic here, not downstream, if there's a type problem.
				panic(f)
			}
			subtypes = append(subtypes, typ)
		}
		f.subtypes = subtypes
	}
	return f.subtypes
}
