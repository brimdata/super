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

	missingFieldsNullable bool
}

func NewSchema(sctx *super.Context) *Schema {
	return &Schema{sctx: sctx}
}

// XXX Remove this when optional fields land.
func NewSchemaWithMissingFieldsAsNullable(sctx *super.Context) *Schema {
	return &Schema{sctx: sctx, missingFieldsNullable: true}
}

// Mixin mixes t into the fused type.
func (s *Schema) Mixin(t super.Type) {
	if s.typ == nil {
		s.typ = t
	} else {
		s.typ = s.merge(s.typ, t)
	}
}

// Type returns the fused type.
func (s *Schema) Type() super.Type {
	return s.typ
}

func (s *Schema) merge(a, b super.Type) super.Type {
	if a == b {
		return a
	}
	aUnder := super.TypeUnder(a)
	bUnder := super.TypeUnder(b)
	if a, ok := aUnder.(*super.TypeRecord); ok {
		if b, ok := bUnder.(*super.TypeRecord); ok {
			fields := slices.Clone(a.Fields)
			if s.missingFieldsNullable {
				for _, f := range b.Fields {
					i, ok := indexOfField(fields, f.Name)
					if !ok {
						i = len(fields)
						fields = append(fields, super.NewField(f.Name, super.TypeNull, f.Opt))
					}
					fields[i].Type = s.merge(fields[i].Type, f.Type)
				}
				for i, f := range fields {
					if _, ok := indexOfField(b.Fields, f.Name); !ok {
						fields[i].Type = s.merge(fields[i].Type, super.TypeNull)
					}
				}
				return s.sctx.MustLookupTypeRecord(fields)
			}
			for _, f := range b.Fields {
				if i, ok := indexOfField(fields, f.Name); !ok {
					fields = append(fields, f)
				} else if fields[i] != f {
					fields[i].Type = s.merge(fields[i].Type, f.Type)
				}
			}
			return s.sctx.MustLookupTypeRecord(fields)
		}
	}
	if a, ok := aUnder.(*super.TypeArray); ok {
		if b, ok := bUnder.(*super.TypeArray); ok {
			return s.sctx.LookupTypeArray(s.merge(a.Type, b.Type))
		}
		if b, ok := bUnder.(*super.TypeSet); ok {
			return s.sctx.LookupTypeArray(s.merge(a.Type, b.Type))
		}
	}
	if a, ok := aUnder.(*super.TypeSet); ok {
		if b, ok := bUnder.(*super.TypeArray); ok {
			return s.sctx.LookupTypeArray(s.merge(a.Type, b.Type))
		}
		if b, ok := bUnder.(*super.TypeSet); ok {
			return s.sctx.LookupTypeSet(s.merge(a.Type, b.Type))
		}
	}
	if a, ok := aUnder.(*super.TypeMap); ok {
		if b, ok := bUnder.(*super.TypeMap); ok {
			keyType := s.merge(a.KeyType, b.KeyType)
			valType := s.merge(a.ValType, b.ValType)
			return s.sctx.LookupTypeMap(keyType, valType)
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
		types = s.mergeAllRecords(types)
		if len(types) == 1 {
			return types[0]
		}
		return s.sctx.LookupTypeUnion(types)
	}
	if _, ok := bUnder.(*super.TypeUnion); ok {
		return s.merge(b, a)
	}
	// XXX Merge enums?
	return s.sctx.LookupTypeUnion([]super.Type{a, b})
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

func (s *Schema) mergeAllRecords(types []super.Type) []super.Type {
	out := types[:0]
	recIndex := -1
	for _, t := range types {
		if super.IsRecordType(t) {
			if recIndex < 0 {
				recIndex = len(out)
			} else {
				out[recIndex] = s.merge(out[recIndex], t)
				continue
			}
		}
		out = append(out, t)
	}
	return out
}
