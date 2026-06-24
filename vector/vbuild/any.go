package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

type anyBuilder struct {
	typ   *super.TypeFusion
	bytes vector.BytesTable
	types []super.Type
}

func newAnyBuilder(typ *super.TypeFusion) Builder {
	return &anyBuilder{typ: typ, bytes: vector.NewBytesTableEmpty(100)}
}

func (a *anyBuilder) Write(vec vector.Any) {
	var b scode.Builder
	for slot := range vec.Len() {
		val := vector.ValueAt(&b, vec, slot)
		a.bytes.Append(val.Bytes())
		a.types = append(a.types, val.Type())
	}
}

func (a *anyBuilder) Build() vector.Any {
	return vector.NewFusion(a.typ, vector.NewBytes(a.bytes), a.types)
}
