package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type fusionBuilder struct {
	typ      *super.TypeFusion
	vals     Builder
	subtypes []super.Type
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
		types := fusion.Subtypes.Types()
		for _, idx := range view.Index {
			f.subtypes = append(f.subtypes, types[idx])
		}
		return
	}
	fusion := vec.(*vector.Fusion)
	f.vals.Write(fusion.Values)
	f.subtypes = append(f.subtypes, fusion.Subtypes.Types()...)
}

func (f *fusionBuilder) Build() vector.Any {
	return vector.NewFusion(f.typ, f.vals.Build(), f.subtypes)
}
