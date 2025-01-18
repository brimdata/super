package agg

import (
	"fmt"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zson"
)

type Collect struct {
	types []super.Type
	bytes []zcode.Bytes
	size  int
}

var _ Function = (*Collect)(nil)

func (c *Collect) Consume(val super.Value) {
	if !val.IsNull() {
		c.Update(val.Type(), val.Bytes())
	}
}

func (c *Collect) Update(typ super.Type, bytes zcode.Bytes) {
	if union, ok := typ.(*super.TypeUnion); ok {
		typ, bytes = union.Untag(bytes)
	}
	c.types = append(c.types, typ)
	c.bytes = append(c.bytes, slices.Clone(bytes))
	c.size += len(bytes)
	for c.size > MaxValueSize {
		// XXX See issue #1813.  For now we silently discard entries
		// to maintain the size limit.
		//c.MemExceeded++
		c.size -= len(c.bytes[0])
		c.bytes = c.bytes[1:]
		c.types = c.types[1:]
	}
}

func (c *Collect) Result(zctx *super.Context) super.Value {
	if len(c.bytes) == 0 {
		// no values found
		return super.Null
	}
	var b zcode.Builder
	inner := innerType(zctx, slices.Clone(c.types))
	if union, ok := inner.(*super.TypeUnion); ok {
		for i, bytes := range c.bytes {
			super.BuildUnion(&b, union.TagOf(c.types[i]), bytes)
		}
	} else {
		for _, bytes := range c.bytes {
			b.Append(bytes)
		}
	}
	return super.NewValue(zctx.LookupTypeArray(inner), b.Bytes())
}

func innerType(zctx *super.Context, types []super.Type) super.Type {
	types = super.UniqueTypes(types)
	if len(types) == 1 {
		return types[0]
	}
	return zctx.LookupTypeUnion(types)
}

func (c *Collect) ConsumeAsPartial(val super.Value) {
	//XXX These should not be passed in here. See issue #3175
	if len(val.Bytes()) == 0 {
		return
	}
	arrayType, ok := val.Type().(*super.TypeArray)
	if !ok {
		panic(fmt.Errorf("collect partial: partial not an array type: %s", zson.FormatValue(val)))
	}
	typ := arrayType.Type
	for it := val.Iter(); !it.Done(); {
		c.Update(typ, it.Next())
	}
}

func (c *Collect) ResultAsPartial(zctx *super.Context) super.Value {
	return c.Result(zctx)
}
