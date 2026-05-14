package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type arraySetBuilder struct {
	typ     super.Type
	inner   Builder
	offsets []uint32
	len     uint32
}

func newArraySetBuilder(typ super.Type) *arraySetBuilder {
	return &arraySetBuilder{
		typ:     typ,
		inner:   New(super.InnerType(typ)),
		offsets: []uint32{0},
	}
}

func (a *arraySetBuilder) Write(vec vector.Any) {
	n := vec.Len()
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		vec = view.Any
		index = view.Index
	}
	var offsets []uint32
	switch vec := vec.(type) {
	case *vector.Array:
		a.inner.Write(vec.Values)
		offsets = vec.Offsets
	case *vector.Set:
		a.inner.Write(vec.Values)
		offsets = vec.Offsets
	default:
		panic(vec)
	}
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		a.len += offsets[idx+1] - offsets[idx]
		a.offsets = append(a.offsets, a.len)
	}
}

func (a *arraySetBuilder) Build(sctx *super.Context) vector.Any {
	vec := a.inner.Build(sctx)
	switch a.typ.(type) {
	case *super.TypeArray:
		typ := sctx.LookupTypeArray(vec.Type())
		return vector.NewArray(typ, a.offsets, vec)
	case *super.TypeSet:
		typ := sctx.LookupTypeSet(vec.Type())
		return vector.NewSet(typ, a.offsets, vec)
	default:
		panic(a.typ)
	}
}
