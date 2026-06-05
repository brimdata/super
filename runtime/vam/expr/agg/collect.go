package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vbuild"
)

type collect struct {
	builder *vbuild.DynamicBuilder
}

func (c *collect) Consume(vec vector.Any) {
	if vec.Len() == 0 || vec.Kind() == vector.KindNull {
		return
	}
	if c.builder == nil {
		c.builder = vbuild.NewDynamicBuilder()
	}
	c.builder.Write(vec)
}

func (c *collect) Result(sctx *super.Context) vector.Any {
	if c.builder == nil {
		return vector.NewNull(1)
	}
	vec := c.builder.Build()
	if dynamic, ok := vec.(*vector.Dynamic); ok {
		vec = vector.NewUnionFromDynamic(sctx, dynamic)
	}
	atyp := sctx.LookupTypeArray(vec.Type())
	return vector.NewArray(atyp, []uint32{0, vec.Len()}, vec)
}

func (c *collect) ConsumeAsPartial(partial vector.Any) {
	inner := vector.PushView(partial).(*vector.Array).Values
	c.Consume(vector.Deunion(inner))
}

func (c *collect) ResultAsPartial(sctx *super.Context) vector.Any {
	if c.builder == nil {
		atyp := sctx.LookupTypeArray(super.TypeNone)
		return vector.NewArray(atyp, []uint32{0, 0}, vector.NewNone(0))
	}
	return c.Result(sctx)
}
