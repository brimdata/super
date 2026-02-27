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
	a, b = super.TypeUnder(a), super.TypeUnder(b)
	if a == b {
		return a
	}
	switch a := a.(type) {
	case *super.TypeRecord:
		if b, ok := b.(*super.TypeRecord); ok {
			fields := slices.Clone(a.Fields)
			// First change all fields to optional that are in "a" but not in "b".
			for k, field := range fields {
				if _, ok := indexOfField(b.Fields, field.Name); !ok {
					fields[k].Opt = true
				}
			}
			// Now fuse all the fields in "b" that are also in "a" and add the fields
			// that are in "b" but not in "a" as they appear in "b".
			for _, field := range b.Fields {
				i, ok := indexOfField(fields, field.Name)
				if ok {
					fields[i].Type = f.fuse(fields[i].Type, field.Type)
					if field.Opt {
						fields[i].Opt = true
					}
				} else {
					fields = append(fields, super.NewFieldWithOpt(field.Name, field.Type, true))
				}
			}
			return f.sctx.MustLookupTypeRecord(fields)
		}
	case *super.TypeArray:
		if b, ok := b.(*super.TypeArray); ok {
			x := f.fuse(a.Type, b.Type)
			return f.sctx.LookupTypeArray(x)
		}
	case *super.TypeSet:
		if b, ok := b.(*super.TypeSet); ok {
			return f.sctx.LookupTypeSet(f.fuse(a.Type, b.Type))
		}
	case *super.TypeMap:
		if b, ok := b.(*super.TypeMap); ok {
			keyType := f.fuse(a.KeyType, b.KeyType)
			valType := f.fuse(a.ValType, b.ValType)
			return f.sctx.LookupTypeMap(keyType, valType)
		}
	case *super.TypeUnion:
		types := f.fuseIntoUnionTypes(nil, a)
		types = f.fuseIntoUnionTypes(types, b)
		if len(types) == 1 {
			return types[0]
		}
		return f.sctx.LookupTypeUnion(types)
	case *super.TypeEnum:
		if b, ok := b.(*super.TypeEnum); ok {
			var newSymbols []string
			for _, s := range b.Symbols {
				if !slices.Contains(a.Symbols, s) {
					newSymbols = append(newSymbols, s)
				}
			}
			if len(newSymbols) == 0 {
				return a
			}
			symbols := append(slices.Clone(a.Symbols), newSymbols...)
			return f.sctx.LookupTypeEnum(symbols)
		}
	case *super.TypeError:
		if b, ok := b.(*super.TypeError); ok {
			return f.sctx.LookupTypeError(f.fuse(a.Type, b.Type))
		}
	}
	if _, ok := b.(*super.TypeUnion); ok {
		return f.fuse(b, a)
	}
	return f.sctx.LookupTypeUnion([]super.Type{a, b})
}

// fuseIntoUnionTypes fuses typ into types while maintaining the invariant that
// types contains at most one type of each complex kind but no unions.
func (f *Fuser) fuseIntoUnionTypes(types []super.Type, typ super.Type) []super.Type {
	typUnder := super.TypeUnder(typ)
	if u, ok := typUnder.(*super.TypeUnion); ok {
		for _, t := range u.Types {
			types = f.fuseIntoUnionTypes(types, t)
		}
		return types
	}
	typKind := typ.Kind()
	for i, t := range types {
		switch {
		case t == typ:
			return types
		case super.TypeUnder(t) == typUnder:
			types[i] = typUnder
			return types
		case typKind != super.PrimitiveKind && typKind == t.Kind():
			types[i] = f.fuse(t, typ)
			return types
		}
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
