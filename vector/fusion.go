package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Fusion struct {
	Typ    *super.TypeFusion
	Values Any
	// XXX
	// For now, we materilize the entire type values of
	// every value in every super anywhere in the value.
	// This has huge crazy overhead but we're just trying
	// to get the semantics worked out right now.
	// This will be replaced by a single column of typeID
	// ints and lazy loading of these types.
	SubTypes *TypeValue
}

var _ Any = (*Union)(nil)

func NewFusion(typ *super.TypeFusion, vals Any, subTypes *TypeValue) *Fusion {
	return &Fusion{Typ: typ, Values: vals, SubTypes: subTypes}
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
	f.SubTypes.Serialize(b, slot)
	b.EndContainer()
}

func (f *Fusion) Dynamic() *Dynamic {
	// XXX we need a way to make a Dynamic from a Super but we can only
	// do this with an sctx but the current vam design doesn't have sctx's
	// when building vector so it can't be easily added with a bit of refactoring.
	panic("TBD")
}
