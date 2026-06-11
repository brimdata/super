package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type fusionBuilder struct {
	typ      *super.TypeFusion
	vals     Builder
	subtypes fusionTypeBuilder
}

func newFusionBuilder(typ *super.TypeFusion) *fusionBuilder {
	return &fusionBuilder{
		typ:  typ,
		vals: New(typ.Type),
	}
}

func (f *fusionBuilder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	if view, ok := vec.(*vector.View); ok {
		fusion := view.Any.(*vector.Fusion)
		f.vals.Write(vector.Pick(fusion.Values, view.Index))
		f.subtypes.Write(vector.Pick(fusion.Subtypes, view.Index))
		return
	}
	fusion := vec.(*vector.Fusion)
	f.vals.Write(fusion.Values)
	f.subtypes.Write(fusion.Subtypes)
}

func (f *fusionBuilder) Build() vector.Any {
	return &vector.Fusion{
		Typ:      f.typ,
		Values:   f.vals.Build(),
		Subtypes: f.subtypes.Build().(*vector.TypeValue),
	}
}

type fusionTypeBuilder struct {
	sctx  *super.Context
	defs  *super.TypeDefs
	ids   []uint32
	types *genericFlatWriter[super.Type]
}

func (t *fusionTypeBuilder) Write(vec vector.Any) {
	if t.sctx != nil || (t.types == nil && typeSctx(vec) != nil) {
		if t.sctx == nil {
			t.sctx = typeSctx(vec)
			t.defs = super.NewTypeDefs()
		}
		t.writeTypeIDs(vec)
		return
	}
	if t.types == nil {
		t.types = &genericFlatWriter[super.Type]{
			valuesOf: func(vec vector.Any) []super.Type { return vec.(*vector.TypeValue).Types() },
			build: func(vals []super.Type) vector.Any {
				return vector.NewTypeValue(vals)
			},
		}
	}
	t.types.Write(vec)
}

func (t *fusionTypeBuilder) writeTypeIDs(vec vector.Any) {
	switch vec := vec.(type) {
	case *vector.Const:
		types := vec.Any.(*vector.TypeValue)
		defs, ids := types.TypeIDs()
		id := super.NewTypeDefsMerger(t.defs, defs).LookupID(ids[0])
		for range vec.Len() {
			t.ids = append(t.ids, id)
		}
	case *vector.Dict:
		index := vec.Index
		types := vec.Any.(*vector.TypeValue)
		defs, ids := types.TypeIDs()
		merger := super.NewTypeDefsMerger(t.defs, defs)
		for _, slot := range index {
			t.ids = append(t.ids, merger.LookupID(ids[slot]))
		}
	case *vector.View:
		index := vec.Index
		types := vec.Any.(*vector.TypeValue)
		defs, ids := types.TypeIDs()
		merger := super.NewTypeDefsMerger(t.defs, defs)
		for _, slot := range index {
			t.ids = append(t.ids, merger.LookupID(ids[slot]))
		}
	case *vector.TypeValue:
		defs, ids := vec.TypeIDs()
		merger := super.NewTypeDefsMerger(t.defs, defs)
		for _, id := range ids {
			t.ids = append(t.ids, merger.LookupID(id))
		}
	default:
		panic(vec)
	}
}

func (t *fusionTypeBuilder) Build() vector.Any {
	if t.types != nil {
		return t.types.Build()
	}
	return vector.NewTypeValueWithLoader(t.sctx, t)
}

func (t *fusionTypeBuilder) Load() (*super.TypeDefs, []uint32) {
	return t.defs, t.ids
}

func typeSctx(vec vector.Any) *super.Context {
	switch vec := vec.(type) {
	case *vector.Const:
		return typeSctx(vec.Any)
	case *vector.Dict:
		return typeSctx(vec.Any)
	case *vector.View:
		return typeSctx(vec.Any)
	case *vector.TypeValue:
		return vec.Sctx()
	default:
		panic(vec)
	}
}
