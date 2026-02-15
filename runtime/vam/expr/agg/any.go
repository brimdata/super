package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

type Any struct {
	val super.Value
}

func NewAny() *Any {
	return &Any{val: super.Null}
}

func (a *Any) Consume(vec vector.Any) {
	if !a.val.IsNull() || vec.Kind() == vector.KindNull {
		return
	}
	var b scode.Builder
	vec.Serialize(&b, 0)
	a.val = super.NewValue(vec.Type(), b.Bytes().Body())
}

func (a *Any) ConsumeAsPartial(vec vector.Any) {
	a.Consume(vec)
}

func (a *Any) Result(*super.Context) super.Value {
	return a.val
}

func (a *Any) ResultAsPartial(*super.Context) super.Value {
	return a.Result(nil)
}
