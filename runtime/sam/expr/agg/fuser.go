package agg

import (
	"slices"

	"github.com/brimdata/super"
)

// Fuser constructs a fused supertype for all the types passed to Fuse.
type Fuser struct {
	sctx *super.Context

	typ   super.Type
	types map[super.Type]struct{}

	missingFieldsNullable bool
}

// XXX this is used by type checker but I think we can use the other one
func NewFuser(sctx *super.Context) *Fuser {
	return &Fuser{sctx: sctx, types: make(map[super.Type]struct{})}
}

// XXX Remove this when optional fields land.
func NewFuserWithMissingFieldsAsNullable(sctx *super.Context) *Fuser {
	f := NewFuser(sctx)
	f.missingFieldsNullable = true
	return f
}

func (f *Fuser) Fuse(t super.Type) {
	if _, ok := f.types[t]; ok {
		return
	}
	f.types[t] = struct{}{}
	if f.typ == nil {
		f.typ = t
	} else {
		f.typ = f.fuse(f.typ, t)
	}
}

// Type returns the computed supertype.
func (f *Fuser) Type() super.Type {
	return f.typ
}

func (f *Fuser) fuse(a, b super.Type) super.Type {
	if a == b {
		return a
	}
	aUnder := super.TypeUnder(a)
	bUnder := super.TypeUnder(b)
	if a, ok := aUnder.(*super.TypeRecord); ok {
		if b, ok := bUnder.(*super.TypeRecord); ok {
			fields := slices.Clone(a.Fields)
			if f.missingFieldsNullable {
				for _, field := range b.Fields {
					i, ok := indexOfField(fields, field.Name)
					if !ok {
						i = len(fields)
						fields = append(fields, super.NewField(field.Name, super.TypeNull))
					}
					fields[i].Type = f.fuse(fields[i].Type, field.Type)
				}
				for i, field := range fields {
					if _, ok := indexOfField(b.Fields, field.Name); !ok {
						fields[i].Type = f.fuse(fields[i].Type, super.TypeNull)
					}
				}
				return f.sctx.MustLookupTypeRecord(fields)
			}
			for _, field := range b.Fields {
				if i, ok := indexOfField(fields, field.Name); !ok {
					fields = append(fields, field)
				} else if fields[i] != field {
					fields[i].Type = f.fuse(fields[i].Type, field.Type)
				}
			}
			return f.sctx.MustLookupTypeRecord(fields)
		}
	}
	if a, ok := aUnder.(*super.TypeArray); ok {
		if b, ok := bUnder.(*super.TypeArray); ok {
			return f.sctx.LookupTypeArray(f.fuse(a.Type, b.Type))
		}
		if b, ok := bUnder.(*super.TypeSet); ok {
			return f.sctx.LookupTypeArray(f.fuse(a.Type, b.Type))
		}
	}
	if a, ok := aUnder.(*super.TypeSet); ok {
		if b, ok := bUnder.(*super.TypeArray); ok {
			return f.sctx.LookupTypeArray(f.fuse(a.Type, b.Type))
		}
		if b, ok := bUnder.(*super.TypeSet); ok {
			return f.sctx.LookupTypeSet(f.fuse(a.Type, b.Type))
		}
	}
	if a, ok := aUnder.(*super.TypeMap); ok {
		if b, ok := bUnder.(*super.TypeMap); ok {
			keyType := f.fuse(a.KeyType, b.KeyType)
			valType := f.fuse(a.ValType, b.ValType)
			return f.sctx.LookupTypeMap(keyType, valType)
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
		types = f.fuseAllRecords(types)
		if len(types) == 1 {
			return types[0]
		}
		return f.sctx.LookupTypeUnion(types)
	}
	if _, ok := bUnder.(*super.TypeUnion); ok {
		return f.fuse(b, a)
	}
	// XXX Merge enums?
	return f.sctx.LookupTypeUnion([]super.Type{a, b})
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

// fuseAllRecords preserves the invariant that any union has a single
// fused record.  It looks through the types argument and for all record types
// fuses them into a common type leaving the single fused record type
// in the returned slice along with all non-record types unchanged.
func (f *Fuser) fuseAllRecords(types []super.Type) []super.Type {
	out := types[:0]
	recIndex := -1
	for _, t := range types {
		if super.IsRecordType(t) {
			if recIndex < 0 {
				recIndex = len(out)
			} else {
				out[recIndex] = f.fuse(out[recIndex], t)
				continue
			}
		}
		out = append(out, t)
	}
	return out
}
