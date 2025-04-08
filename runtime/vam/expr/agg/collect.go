package agg

import (
	"github.com/brimdata/super"
	samagg "github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type collect struct {
	samcollect samagg.Collect
}

func newCollect() *collect {
	return &collect{}
}

func (c *collect) Consume(vec vector.Any) {
	typ := vec.Type()
	nulls := vector.NullsOf(vec)
	var b zcode.Builder
	for i := range vec.Len() {
		if nulls.Value(i) {
			continue
		}
		b.Truncate()
		vec.Serialize(&b, i)
		c.samcollect.Consume(super.NewValue(typ, b.Bytes().Body()))
	}
}

func (c *collect) Result(sctx *super.Context) super.Value {
	return c.samcollect.Result(sctx)
}

func (c *collect) ConsumeAsPartial(partial vector.Any) {
	if c, ok := partial.(*vector.Const); ok && c.Value().IsNull() {
		return
	}
	array, ok := partial.(*vector.Array)
	if !ok {
		panic("collection: partial not an array type")
	}
	var b zcode.Builder
	typ := array.Values.Type()
	for i := range array.Len() {
		for k := array.Offsets[i]; k < array.Offsets[i+1]; k++ {
			b.Truncate()
			array.Values.Serialize(&b, k)
			c.samcollect.Consume(super.NewValue(typ, b.Bytes().Body()))
		}
	}
}

func (c *collect) ResultAsPartial(sctx *super.Context) super.Value {
	return c.samcollect.ResultAsPartial(sctx)
}
