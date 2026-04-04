package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type count struct {
	count int64
}

func (c *count) Consume(vec vector.Any) {
	vector.Apply(true, c.consume, vec)
}

func (a *count) consume(vecs ...vector.Any) vector.Any {
	vec := vecs[0]
	if vec.Kind() == vector.KindNull {
		return vec
	}
	a.count += int64(vec.Len())
	return vec
}

func (a *count) Result(*super.Context) super.Value {
	return super.NewInt64(a.count)
}

func (a *count) ConsumeAsPartial(partial vector.Any) {
	if partial.Len() != 1 || partial.Type() != super.TypeInt64 {
		panic("count: bad partial")
	}
	a.count += vector.IntValue(partial, 0)
}

func (a *count) ResultAsPartial(*super.Context) super.Value {
	return a.Result(nil)
}
