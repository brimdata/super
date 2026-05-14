package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type enumBuilder struct {
	typ  *super.TypeEnum
	uint Builder
}

func newEnumBuilder(typ *super.TypeEnum) Builder {
	return &enumBuilder{
		typ:  typ,
		uint: New(super.TypeUint64),
	}
}

func (a *enumBuilder) Write(vec vector.Any) {
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		index = view.Index
		vec = view.Any
	}
	var vals vector.Any = vec.(*vector.Enum).Uint
	if index != nil {
		vals = vector.Pick(vals, index)
	}
	a.uint.Write(vals)
}

func (a *enumBuilder) Build(sctx *super.Context) vector.Any {
	return vector.NewEnum(a.typ, a.uint.Build(sctx).(*vector.Uint).Values)
}
