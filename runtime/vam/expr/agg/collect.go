package agg

import (
	"github.com/brimdata/super"
	samagg "github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

type collect struct {
	samcollect samagg.Collect
}

func (c *collect) Consume(vec vector.Any) {
	if _, ok := vec.(*vector.Error); ok {
		return
	}
	typ := vec.Type()
	nulls := vector.NullsOf(vec)
	var b scode.Builder
	for i := range vec.Len() {
		if nulls.IsSet(i) {
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
	n := partial.Len()
	var index []uint32
	if view, ok := partial.(*vector.View); ok {
		partial, index = view.Any, view.Index
	}
	array, ok := partial.(*vector.Array)
	if !ok {
		panic("collection: partial not an array type")
	}
	var b scode.Builder
	typ := array.Values.Type()
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		for k := array.Offsets[idx]; k < array.Offsets[idx+1]; k++ {
			b.Truncate()
			array.Values.Serialize(&b, k)
			c.samcollect.Consume(super.NewValue(typ, b.Bytes().Body()))
		}
	}
}

func (c *collect) ResultAsPartial(sctx *super.Context) super.Value {
	return c.samcollect.ResultAsPartial(sctx)
}
