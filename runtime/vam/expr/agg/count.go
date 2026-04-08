package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type count struct {
	count int64
}

func (a *count) Consume(vec vector.Any) {
	if vec.Kind() == vector.KindNull {
		return
	}
	a.count += int64(vec.Len())
}

func (a *count) Result(*super.Context) vector.Any {
	return vector.NewInt(super.TypeInt64, []int64{a.count})
}

func (a *count) ConsumeAsPartial(partial vector.Any) {
	if partial.Len() != 1 || partial.Type() != super.TypeInt64 {
		panic("count: bad partial")
	}
	a.count += vector.IntValue(partial, 0)
}

func (a *count) ResultAsPartial(*super.Context) vector.Any {
	return a.Result(nil)
}
