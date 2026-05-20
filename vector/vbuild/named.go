package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type namedBuilder struct {
	typ  *super.TypeNamed
	vals Builder
}

func newNamedBuilder(typ *super.TypeNamed) Builder {
	return &namedBuilder{typ: typ}
}

func (n *namedBuilder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	if n.vals == nil {
		n.vals = New(n.typ.Type)
	}
	n.vals.Write(vec.(*vector.Named).Any)
}

func (n *namedBuilder) Build() vector.Any {
	var vec vector.Any
	if n.vals != nil {
		vec = n.vals.Build()
	} else {
		vec = vector.NewEmpty(n.typ.Type)
	}
	return vector.NewNamed(n.typ, vec)
}
