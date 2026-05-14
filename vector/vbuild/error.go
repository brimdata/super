package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type errorBuilder struct {
	vals Builder
}

func (e *errorBuilder) Write(vec vector.Any) {
	e.vals.Write(vec.(*vector.Error).Vals)
}

func (e *errorBuilder) Build(sctx *super.Context) vector.Any {
	vals := e.vals.Build(sctx)
	return vector.NewError(sctx.LookupTypeError(vals.Type()), vals)
}
