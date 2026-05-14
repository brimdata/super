package vbuild

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type unionBuilder struct {
	typ *super.TypeUnion

	builder *DynamicBuilder
}

func (u *unionBuilder) Write(vec vector.Any) {
	// Assert all incoming types in union.
	vec = vector.Deunion(vec)
	check := []vector.Any{vec}
	if d, ok := vec.(*vector.Dynamic); ok {
		check = d.Values
	}
	bad := slices.ContainsFunc(check, func(vec vector.Any) bool {
		return u.typ.TagOf(vec.Type()) == -1
	})
	if bad {
		panic("incoming vector contains values not in union")
	}
	u.builder.Write(vec)
}

func (u *unionBuilder) Build(sctx *super.Context) vector.Any {
	d := u.builder.build(sctx)
	return vector.NewUnion(u.typ, d.Tags, d.Values)
}
