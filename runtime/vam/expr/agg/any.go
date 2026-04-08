package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Any struct {
	result vector.Any
}

func NewAny() *Any {
	return &Any{}
}

func (a *Any) Consume(vec vector.Any) {
	if a.result != nil || vec.Kind() == vector.KindNull {
		return
	}
	a.result = vector.Pick(vec, []uint32{0})
}

func (a *Any) ConsumeAsPartial(vec vector.Any) {
	a.Consume(vec)
}

func (a *Any) Result(sctx *super.Context) vector.Any {
	if a.result == nil {
		return vector.NewNull(1)
	}
	return a.result
}

func (a *Any) ResultAsPartial(*super.Context) vector.Any {
	return a.Result(nil)
}
