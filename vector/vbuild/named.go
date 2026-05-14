package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type namedBuilder struct {
	name string
	vals Builder
}

func (n *namedBuilder) Write(vec vector.Any) {
	n.vals.Write(vec.(*vector.Named).Any)
}

func (n *namedBuilder) Build(sctx *super.Context) vector.Any {
	vals := n.vals.Build(sctx)
	typ, err := sctx.LookupTypeNamed(n.name, vals.Type())
	if err != nil {
		panic(err)
	}
	return vector.NewNamed(typ, vals)
}
