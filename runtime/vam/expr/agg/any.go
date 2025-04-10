package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type Any struct {
	val super.Value
}

func NewAny() *Any {
	return &Any{val: super.Null}
}

func (a *Any) Consume(vec vector.Any) {
	isnull := a.val.IsNull()
	if !isnull {
		return
	}
	nulls := vector.NullsOf(vec)
	for i := range vec.Len() {
		if a.val == super.Null || (isnull && !nulls.IsSet(i)) {
			var b zcode.Builder
			vec.Serialize(&b, i)
			a.val = super.NewValue(vec.Type(), b.Bytes().Body())
			isnull = a.val.IsNull()
		}
	}
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
