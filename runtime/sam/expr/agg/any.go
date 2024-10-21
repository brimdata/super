package agg

import (
	"github.com/brimdata/super"
)

type Any super.Value

var _ Function = (*Any)(nil)

func NewAny() *Any {
	a := (Any)(super.Null)
	return &a
}

func (a *Any) Consume(val super.Value) {
	// Copy any value from the input while favoring any-typed non-null values
	// over null values.
	if (*super.Value)(a).Type() == nil || (*super.Value)(a).IsNull() && !val.IsNull() {
		*a = Any(val.Copy())
	}
}

func (a *Any) Result(*super.Context) super.Value {
	if (*super.Value)(a).Type() == nil {
		return super.Null
	}
	return *(*super.Value)(a)
}

func (a *Any) ConsumeAsPartial(v super.Value) {
	a.Consume(v)
}

func (a *Any) ResultAsPartial(*super.Context) super.Value {
	return a.Result(nil)
}
