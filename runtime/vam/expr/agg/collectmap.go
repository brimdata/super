package agg

import (
	"github.com/brimdata/super"
	samagg "github.com/brimdata/super/runtime/sam/expr/agg"
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
	if vec.Kind() == vector.KindError {
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
		c.samCollectMap.Consume(super.NewValue(typ, b.Bytes().Body()))
	}
}

func (c *collectMap) Result(sctx *super.Context) super.Value {
	return c.samCollectMap.Result(sctx)
}

func (c *collectMap) ConsumeAsPartial(partial vector.Any) {
	c.Consume(partial)
}

func (c *collectMap) ResultAsPartial(sctx *super.Context) super.Value {
	return c.samCollectMap.ResultAsPartial(sctx)
}
