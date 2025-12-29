package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type count struct {
	count int64
}

func (a *count) Consume(vec vector.Any) {
	if c, ok := vec.(*vector.Const); ok && c.Value().IsNull() {
		return
	}
	if _, ok := vector.Under(vec).Type().(*super.TypeError); ok {
		return
	}
	a.count += int64((vec.Len()) - vector.NullsOf(vec).TrueCount())
}

func (a *count) Result(*super.Context) super.Value {
	return super.NewInt64(a.count)
}

func (a *count) ConsumeAsPartial(partial vector.Any) {
	if partial.Len() != 1 || partial.Type() != super.TypeInt64 {
		panic("count: bad partial")
	}
	count, _ := vector.IntValue(partial, 0)
	a.count += count
}

func (a *count) ResultAsPartial(*super.Context) super.Value {
	return a.Result(nil)
}
