package agg

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

type Collect struct {
	values []super.Value
	size   int
}

var _ Function = (*Collect)(nil)

func (c *Collect) Consume(val super.Value) {
	if val.IsNull() {
		return
	}
	c.values = append(c.values, val.Under().Copy())
	c.size += len(val.Bytes())
	for c.size > MaxValueSize {
		// XXX See issue #1813.  For now we silently discard entries
		// to maintain the size limit.
		//c.MemExceeded++
		c.size -= len(c.values[0].Bytes())
		c.values = c.values[1:]
	}
}

func (c *Collect) Result(sctx *super.Context) super.Value {
	if len(c.values) == 0 {
		// no values found
		return super.Null
	}
	var b scode.Builder
	inner := innerType(sctx, c.values)
	if union, ok := inner.(*super.TypeUnion); ok {
		for _, val := range c.values {
			super.BuildUnion(&b, union.TagOf(val.Type()), val.Bytes())
		}
	} else {
		for _, val := range c.values {
			b.Append(val.Bytes())
		}
	}
	return super.NewValue(sctx.LookupTypeArray(inner), b.Bytes())
}

func innerType(sctx *super.Context, vals []super.Value) super.Type {
	var types []super.Type
	for _, val := range vals {
		types = append(types, val.Type())
	}
	types = super.UniqueTypes(types)
	if len(types) == 1 {
		return types[0]
	}
	return sctx.LookupTypeUnion(types)
}

func (c *Collect) ConsumeAsPartial(val super.Value) {
	//XXX These should not be passed in here. See issue #3175
	if len(val.Bytes()) == 0 {
		return
	}
	arrayType, ok := val.Type().(*super.TypeArray)
	if !ok {
		panic(fmt.Errorf("collect partial: partial not an array type: %s", sup.FormatValue(val)))
	}
	typ := arrayType.Type
	for it := val.Iter(); !it.Done(); {
		c.Consume(super.NewValue(typ, it.Next()))
	}
}

func (c *Collect) ResultAsPartial(sctx *super.Context) super.Value {
	return c.Result(sctx)
}
