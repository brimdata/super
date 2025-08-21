package agg

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type CollectMap struct {
	entries map[string]mapEntry
	scratch []byte
}

func newCollectMap() *CollectMap {
	return &CollectMap{entries: make(map[string]mapEntry)}
}

var _ Function = (*Collect)(nil)

type mapEntry struct {
	key super.Value
	val super.Value
}

func (c *CollectMap) Consume(val super.Value) {
	if val.IsNull() {
		return
	}
	mtyp, ok := super.TypeUnder(val.Type()).(*super.TypeMap)
	if !ok {
		return
	}
	// Copy val.Bytes since we're going to keep slices of it.
	it := scode.Iter(slices.Clone(val.Bytes()))
	for !it.Done() {
		keyTagAndBody := it.NextTagAndBody()
		key := valueUnder(mtyp.KeyType, keyTagAndBody.Body())
		val := valueUnder(mtyp.ValType, it.Next())
		c.scratch = super.AppendTypeValue(c.scratch[:0], key.Type())
		c.scratch = append(c.scratch, keyTagAndBody...)
		// This will squash existing values which is what we want.
		c.entries[string(c.scratch)] = mapEntry{key, val}
	}
}

func (c *CollectMap) ConsumeAsPartial(val super.Value) {
	c.Consume(val)
}

func (c *CollectMap) Result(sctx *super.Context) super.Value {
	if len(c.entries) == 0 {
		return super.Null
	}
	var ktypes, vtypes []super.Type
	for _, e := range c.entries {
		ktypes = append(ktypes, e.key.Type())
		vtypes = append(vtypes, e.val.Type())
	}
	// Keep track of number of unique types in collection. If there is only one
	// unique type we don't build a union for each value (though the base type could
	// be a union itself).
	ktyp, kuniq := unionOf(sctx, ktypes)
	vtyp, vuniq := unionOf(sctx, vtypes)
	var builder scode.Builder
	for _, e := range c.entries {
		appendMapVal(&builder, ktyp, e.key, kuniq)
		appendMapVal(&builder, vtyp, e.val, vuniq)
	}
	typ := sctx.LookupTypeMap(ktyp, vtyp)
	b := super.NormalizeMap(builder.Bytes())
	return super.NewValue(typ, b)
}

func (c *CollectMap) ResultAsPartial(sctx *super.Context) super.Value {
	return c.Result(sctx)
}

func appendMapVal(b *scode.Builder, typ super.Type, val super.Value, uniq int) {
	if uniq > 1 {
		u := super.TypeUnder(typ).(*super.TypeUnion)
		super.BuildUnion(b, u.TagOf(val.Type()), val.Bytes())
	} else {
		b.Append(val.Bytes())
	}
}

func unionOf(sctx *super.Context, types []super.Type) (super.Type, int) {
	types = super.UniqueTypes(types)
	if len(types) == 1 {
		return types[0], 1
	}
	return sctx.LookupTypeUnion(types), len(types)
}

// valueUnder is like super.(*Value).Under but it preserves non-union named types.
func valueUnder(typ super.Type, b scode.Bytes) super.Value {
	val := super.NewValue(typ, b)
	if _, ok := super.TypeUnder(typ).(*super.TypeUnion); !ok {
		return val
	}
	return val.Under()
}
