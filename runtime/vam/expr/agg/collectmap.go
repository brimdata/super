package agg

import (
	"github.com/brimdata/super"
	samagg "github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

type collectMap struct {
	samCollectMap *samagg.CollectMap
}

func newCollectMap() *collectMap {
	return &collectMap{samagg.NewCollectMap()}
}

func (c *collectMap) Consume(vec vector.Any) {
	if k := vec.Kind(); k == vector.KindNull || k == vector.KindError {
		return
	}
	typ := vec.Type()
	var b scode.Builder
	for i := range vec.Len() {
		b.Truncate()
		vec.Serialize(&b, i)
		c.samCollectMap.Consume(super.NewValue(typ, b.Bytes().Body()))
	}
}

func (c *collectMap) Result(sctx *super.Context) vector.Any {
	val := c.samCollectMap.Result(sctx)
	return sbuf.Dematerialize(sctx, sbuf.NewArray([]super.Value{val}))
}

func (c *collectMap) ConsumeAsPartial(partial vector.Any) {
	c.Consume(partial)
}

func (c *collectMap) ResultAsPartial(sctx *super.Context) vector.Any {
	val := c.samCollectMap.ResultAsPartial(sctx)
	return sbuf.Dematerialize(sctx, sbuf.NewArray([]super.Value{val}))
}
