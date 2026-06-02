package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type errorBuilder struct {
	typ  *super.TypeError
	vals Builder
}

func (e *errorBuilder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	e.vals.Write(vec.(*vector.Error).Vals)
}

func (e *errorBuilder) Build() vector.Any {
	return vector.NewError(e.typ, e.vals.Build())
}
