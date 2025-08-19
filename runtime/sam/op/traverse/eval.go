package traverse

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zcode"
)

func combine(sctx *super.Context, batches []zbuf.Batch) super.Value {
	switch len(batches) {
	case 0:
		return super.Null
	case 1:
		return makeArray(sctx, batches[0].Values())
	default:
		var vals []super.Value
		for _, batch := range batches {
			vals = append(vals, batch.Values()...)
		}
		return makeArray(sctx, vals)
	}
}

func makeArray(sctx *super.Context, vals []super.Value) super.Value {
	if len(vals) == 0 {
		return super.Null
	}
	if len(vals) == 1 {
		return vals[0]
	}
	typ := vals[0].Type()
	for _, val := range vals[1:] {
		if typ != val.Type() {
			return makeUnionArray(sctx, vals)
		}
	}
	var b zcode.Builder
	for _, val := range vals {
		b.Append(val.Bytes())
	}
	return super.NewValue(sctx.LookupTypeArray(typ), b.Bytes())
}

func makeUnionArray(sctx *super.Context, vals []super.Value) super.Value {
	types := make(map[super.Type]struct{})
	for _, val := range vals {
		types[val.Type()] = struct{}{}
	}
	utypes := make([]super.Type, 0, len(types))
	for typ := range types {
		utypes = append(utypes, typ)
	}
	union := sctx.LookupTypeUnion(utypes)
	var b zcode.Builder
	for _, val := range vals {
		super.BuildUnion(&b, union.TagOf(val.Type()), val.Bytes())
	}
	return super.NewValue(sctx.LookupTypeArray(union), b.Bytes())
}
