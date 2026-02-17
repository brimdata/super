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

func (s *Fuser) Fuse(t super.Type) {
	if _, ok := s.types[t]; ok {
		return
	}
	s.types[t] = struct{}{}
	if s.typ == nil {
		s.typ = t
	} else {
		s.typ = s.fuse(s.typ, t)
	}
}

// Type returns the computed supertype.
func (s *Fuser) Type() super.Type {
	return s.typ
}

func (s *Fuser) fuse(a, b super.Type) super.Type {
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
						fields = append(fields, super.NewField(f.Name, super.TypeNull))
					}
					fields[i].Type = s.fuse(fields[i].Type, f.Type)
				}
				for i, f := range fields {
					if _, ok := indexOfField(b.Fields, f.Name); !ok {
						fields[i].Type = s.fuse(fields[i].Type, super.TypeNull)
					}
				}
				return s.sctx.MustLookupTypeRecord(fields)
			}
			for _, f := range b.Fields {
				if i, ok := indexOfField(fields, f.Name); !ok {
					fields = append(fields, f)
				} else if fields[i] != f {
					fields[i].Type = s.fuse(fields[i].Type, f.Type)
				}
			}
			return s.sctx.MustLookupTypeRecord(fields)
		}
	}
	if a, ok := aUnder.(*super.TypeArray); ok {
		if b, ok := bUnder.(*super.TypeArray); ok {
			return s.sctx.LookupTypeArray(s.fuse(a.Type, b.Type))
		}
		if b, ok := bUnder.(*super.TypeSet); ok {
			return s.sctx.LookupTypeArray(s.fuse(a.Type, b.Type))
		}
	}
	if a, ok := aUnder.(*super.TypeSet); ok {
		if b, ok := bUnder.(*super.TypeArray); ok {
			return s.sctx.LookupTypeArray(s.fuse(a.Type, b.Type))
		}
		if b, ok := bUnder.(*super.TypeSet); ok {
			return s.sctx.LookupTypeSet(s.fuse(a.Type, b.Type))
		}
	}
	if a, ok := aUnder.(*super.TypeMap); ok {
		if b, ok := bUnder.(*super.TypeMap); ok {
			keyType := s.fuse(a.KeyType, b.KeyType)
			valType := s.fuse(a.ValType, b.ValType)
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
		types = s.fuseAllRecords(types)
		if len(types) == 1 {
			return types[0]
		}
		return s.sctx.LookupTypeUnion(types)
	}
	if _, ok := bUnder.(*super.TypeUnion); ok {
		return s.fuse(b, a)
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

// fuseAllRecords preserve the invarient that any union has a single
// fused record.  It looks through the types argument and for all record types
// fuses them into a common type leaving the single fused record type
// in the returned slice along with all non-record types unchanged.
func (s *Fuser) fuseAllRecords(types []super.Type) []super.Type {
	out := types[:0]
	recIndex := -1
	for _, t := range types {
		if super.IsRecordType(t) {
			if recIndex < 0 {
				recIndex = len(out)
			} else {
				out[recIndex] = s.fuse(out[recIndex], t)
				continue
			}
		}
		out = append(out, t)
	}
	return out
}
