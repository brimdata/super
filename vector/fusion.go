package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Fusion struct {
	Sctx   *super.Context
	Typ    *super.TypeFusion
	Values Any
	// The TypeValue vectors are always created as typedefs, whether
	// loaded from CSUP, built from a fuse operation, built from FJSON
	// etc.  They are only materialized into the query context when
	// absoluately necessary to perform less common operations that require
	// detailed types.  When coming from CSUP, the typedefs are lazily loaded
	// and often never even read from storage.
	Subtypes *TypeValue
}

var _ Any = (*Fusion)(nil)

func NewFusion(sctx *super.Context, typ *super.TypeFusion, vals Any, subtypes []super.Type) *Fusion {
	return &Fusion{Sctx: sctx, Typ: typ, Values: vals, Subtypes: NewTypeValue(sctx, subtypes)}
}

func NewFusionWithLoader(sctx *super.Context, typ *super.TypeFusion, loader TypesLoader, vals Any) *Fusion {
	return &Fusion{Sctx: sctx, Typ: typ, Values: vals, Subtypes: NewTypeValueWithLoader(sctx, loader)}
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
	f.Subtypes.Serialize(b, slot)
	b.EndContainer()
}
