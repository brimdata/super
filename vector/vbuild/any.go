package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type anyBuilder struct {
	typ      *super.TypeFusion
	bytes    Builder
	subtypes fusionTypeBuilder
}

func newAnyBuilder(typ *super.TypeFusion) Builder {
	return &anyBuilder{typ: typ, bytes: newBytesBuilder(super.TypeBytes)}
}

func (a *anyBuilder) Write(vec vector.Any) {
	fusion := vector.PushView(vec).(*vector.Fusion)
	a.bytes.Write(fusion.Values)
	a.subtypes.Write(fusion.Subtypes)
}

func (a *anyBuilder) Build() vector.Any {
	return &vector.Fusion{
		Typ:      a.typ,
		Values:   a.bytes.Build(),
		Subtypes: a.subtypes.Build().(*vector.TypeValue),
	}
}
