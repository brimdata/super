package agg

import (
	"github.com/superdb/super"
	"github.com/superdb/super/scode"
)

// newArray returns an array of vals. If vals is empty, newArray returns
// super.Null.
func newArray(sctx *super.Context, vals []super.Value) super.Value {
	if len(vals) == 0 {
		return super.Null
	}
	var types []super.Type
	for _, val := range vals {
		types = append(types, val.Type())
	}
	types = super.Flatten(super.UniqueTypes(types))
	var b scode.Builder
	var typ super.Type
	if len(types) == 1 {
		for _, val := range vals {
			b.Append(val.Bytes())
		}
		typ = types[0]
	} else {
		union := sctx.MustLookupTypeUnion(types)
		for _, val := range vals {
			val = val.Deunion()
			super.BuildUnion(&b, union.TagOf(val.Type()), val.Bytes())
		}
		typ = union
	}
	return super.NewValue(sctx.LookupTypeArray(typ), b.Bytes())
}
