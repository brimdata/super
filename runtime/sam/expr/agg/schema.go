package agg

import (
	"slices"

	"github.com/brimdata/super"
)

// Schema constructs a fused type for types passed to Mixin.  Values of any
// mixed-in type can be shaped to the fused type without loss of information.
type Schema struct {
	sctx *super.Context

	typ super.Type
}

func NewSchema(sctx *super.Context) *Schema {
	return &Schema{sctx: sctx}
}

// Mixin mixes t into the fused type.
func (s *Schema) Mixin(t super.Type) {
	if s.typ == nil {
		s.typ = t
	} else {
		s.typ = merge(s.sctx, s.typ, t)
	}
}

// Type returns the fused type.
func (s *Schema) Type() super.Type {
	return s.typ
}

func merge(sctx *super.Context, a, b super.Type) super.Type {
	if a == b {
		return a
	}
	aUnder := super.TypeUnder(a)
	if aUnder == super.TypeNull {
		return b
	}
	bUnder := super.TypeUnder(b)
	if bUnder == super.TypeNull {
		return a
	}
	if a, ok := aUnder.(*super.TypeRecord); ok {
		if b, ok := bUnder.(*super.TypeRecord); ok {
			fields := slices.Clone(a.Fields)
			for _, f := range b.Fields {
				if i, ok := indexOfField(fields, f.Name); !ok {
					fields = append(fields, f)
				} else if fields[i] != f {
					fields[i].Type = merge(sctx, fields[i].Type, f.Type)
				}
			}
			return sctx.MustLookupTypeRecord(fields)
		}
	}
	if a, ok := aUnder.(*super.TypeArray); ok {
		if b, ok := bUnder.(*super.TypeArray); ok {
			return sctx.LookupTypeArray(merge(sctx, a.Type, b.Type))
		}
		if b, ok := bUnder.(*super.TypeSet); ok {
			return sctx.LookupTypeArray(merge(sctx, a.Type, b.Type))
		}
	}
	if a, ok := aUnder.(*super.TypeSet); ok {
		if b, ok := bUnder.(*super.TypeArray); ok {
			return sctx.LookupTypeArray(merge(sctx, a.Type, b.Type))
		}
		if b, ok := bUnder.(*super.TypeSet); ok {
			return sctx.LookupTypeSet(merge(sctx, a.Type, b.Type))
		}
	}
	if a, ok := aUnder.(*super.TypeMap); ok {
		if b, ok := bUnder.(*super.TypeMap); ok {
			keyType := merge(sctx, a.KeyType, b.KeyType)
			valType := merge(sctx, a.ValType, b.ValType)
			return sctx.LookupTypeMap(keyType, valType)
		}
	}
	if a, ok := aUnder.(*super.TypeUnion); ok {
		types := slices.Clone(a.Types)
		if bUnion, ok := bUnder.(*super.TypeUnion); ok {
			for _, t := range bUnion.Types {
				types = appendIfAbsent(types, t)
			}
		} else {
			types = appendIfAbsent(types, b)
		}
		types = mergeAllRecords(sctx, types)
		if len(types) == 1 {
			return types[0]
		}
		return sctx.LookupTypeUnion(types)
	}
	if _, ok := bUnder.(*super.TypeUnion); ok {
		return merge(sctx, b, a)
	}
	// XXX Merge enums?
	return sctx.LookupTypeUnion([]super.Type{a, b})
}

func appendIfAbsent(types []super.Type, typ super.Type) []super.Type {
	if slices.Contains(types, typ) {
		return types
	}
	return append(types, typ)
}

func indexOfField(fields []super.Field, name string) (int, bool) {
	for i, f := range fields {
		if f.Name == name {
			return i, true
		}
	}
	return -1, false
}

func mergeAllRecords(sctx *super.Context, types []super.Type) []super.Type {
	out := types[:0]
	recIndex := -1
	for _, t := range types {
		if super.IsRecordType(t) {
			if recIndex < 0 {
				recIndex = len(out)
			} else {
				out[recIndex] = merge(sctx, out[recIndex], t)
				continue
			}
		}
		out = append(out, t)
	}
	return out
}
