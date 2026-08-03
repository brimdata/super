package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vbuild"
)

type collect struct {
	builder *vbuild.DynamicBuilder
	defuse  *expr.Defuse
}

func newCollect(sctx *super.Context) *collect {
	return &collect{defuse: expr.NewDefuse(sctx)}
}

func (c *collect) NoRip() bool { return true }

func (c *collect) Consume(vec vector.Any) {
	vector.Apply(vector.ApplyRipUnions, c.consume, c.defuse.Eval(vec))
}

func (c *collect) consume(vecs ...vector.Any) vector.Any {
	vec := vecs[0]
	if vec.Kind() == vector.KindNone || vec.Len() == 0 {
		return vector.NewNull(vecs[0].Len())
	}
	if c.builder == nil {
		c.builder = vbuild.NewDynamicBuilder()
	}
	c.builder.Write(vec)
	return vector.NewNone(vecs[0].Len())
}

func (c *collect) Result(sctx *super.Context) vector.Any {
	if c.builder == nil {
		atyp := sctx.LookupTypeArray(super.TypeNone)
		return vector.NewArray(atyp, []uint32{0, 0}, vector.NewNone(0))
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
	return c.Result(sctx)
}

// arrayAgg is the same as collect, except if nothing was collected it returns
// null instead of an empty array.
type arrayAgg struct {
	collect
}

func (a *arrayAgg) Result(sctx *super.Context) vector.Any {
	if a.collect.builder == nil {
		return vector.NewNull(1)
	}
	return a.collect.Result(sctx)
}
