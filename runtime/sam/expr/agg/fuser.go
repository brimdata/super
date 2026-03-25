package agg

import (
	"slices"

	"github.com/brimdata/super"
)

// Fuser constructs a fused supertype for all the types passed to Fuse.
type Fuser struct {
	sctx     *super.Context
	complete bool

	typ   super.Type
	types map[super.Type]struct{}
}

// XXX this is used by type checker but I think we can use the other one
func NewFuser(sctx *super.Context, complete bool) *Fuser {
	return &Fuser{sctx: sctx, complete: complete, types: make(map[super.Type]struct{})}
}

func (f *Fuser) Fuse(t super.Type) {
	if _, ok := f.types[t]; ok {
		return
	}
	f.types[t] = struct{}{}
	if f.typ == nil {
		f.typ = f.fuseMono(t)
	} else {
		f.typ = f.fuse(f.typ, t)
	}
}

// Type returns the computed supertype.
func (f *Fuser) Type() super.Type {
	return f.typ
}

func (f *Fuser) fuse(a, b super.Type) super.Type {
	a, b = super.TypeUnder(a), super.TypeUnder(b)
	if a == b {
		return a
	}
	if typ, ok := a.(*super.TypeFusion); ok {
		return f.fusion(f.fuse(typ.Type, b))
	}
	if typ, ok := b.(*super.TypeFusion); ok {
		return f.fusion(f.fuse(a, typ.Type))
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
			fusedRec := f.sctx.MustLookupTypeRecord(fields)
			if recChanged(a, fusedRec) || recChanged(b, fusedRec) {
				return f.fusion(fusedRec)
			}
			return fusedRec
		}
	case *super.TypeArray:
		if b, ok := b.(*super.TypeArray); ok {
			return f.fusion(f.sctx.LookupTypeArray(f.fuse(a.Type, b.Type)))
		}
	case *super.TypeSet:
		if b, ok := b.(*super.TypeSet); ok {
			return f.fusion(f.sctx.LookupTypeSet(f.fuse(a.Type, b.Type)))
		}
	case *super.TypeMap:
		if b, ok := b.(*super.TypeMap); ok {
			keyType := f.fuse(a.KeyType, b.KeyType)
			valType := f.fuse(a.ValType, b.ValType)
			return f.fusion(f.sctx.LookupTypeMap(keyType, valType))
		}
	case *super.TypeUnion:
		types := f.fuseIntoUnionTypes(nil, a)
		types = f.fuseIntoUnionTypes(types, b)
		if len(types) == 1 {
			return types[0]
		}
		return f.fusion(f.sctx.LookupTypeUnion(types))
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
			return f.fusion(f.sctx.LookupTypeEnum(symbols))
		}
	case *super.TypeError:
		if b, ok := b.(*super.TypeError); ok {
			return f.fusion(f.sctx.LookupTypeError(f.fuse(a.Type, b.Type)))
		}
	}
	if _, ok := b.(*super.TypeUnion); ok {
		return f.fuse(b, a)
	}
	return f.fusion(f.sctx.LookupTypeUnion([]super.Type{a, b}))
}

func (f *Fuser) fuseMono(typ super.Type) super.Type {
	typ = super.TypeUnder(typ)
	if typ, ok := typ.(*super.TypeFusion); ok {
		return f.fusion(f.fuseMono(typ.Type))
	}
	var out super.Type
	switch typ := typ.(type) {
	case *super.TypeRecord:
		fields := slices.Clone(typ.Fields)
		for i, field := range fields {
			fields[i].Type = f.fuseMono(field.Type)
		}
		out = f.sctx.MustLookupTypeRecord(fields)
	case *super.TypeArray:
		out = f.sctx.LookupTypeArray(f.fuseMono(typ.Type))
	case *super.TypeSet:
		out = f.sctx.LookupTypeSet(f.fuseMono(typ.Type))
	case *super.TypeMap:
		out = f.fusion(f.sctx.LookupTypeMap(f.fuseMono(typ.KeyType), f.fuseMono(typ.ValType)))
	case *super.TypeUnion:
		types := f.fuseIntoUnionTypes(nil, typ)
		if len(types) == 1 {
			out = types[0]
		} else {
			out = f.sctx.LookupTypeUnion(types)
		}
	case *super.TypeEnum:
		return typ
	case *super.TypeError:
		out = f.sctx.LookupTypeError(f.fuseMono(typ.Type))
	case *super.TypeNamed:
		if inner := f.fuseMono(typ.Type); inner != typ.Type {
			// If type changed, drop the name.
			out = inner
		}
	default:
		out = typ
	}
	if out != typ {
		out = f.fusion(out)
	}
	return out
}

// fuseIntoUnionTypes fuses typ into types while maintaining the invariant that
// types contains at most one type of each complex kind but no unions.
func (f *Fuser) fuseIntoUnionTypes(types []super.Type, typ super.Type) []super.Type {
	typUnder := super.TypeUnder(typ)
	switch typ := typUnder.(type) {
	case *super.TypeUnion:
		for _, t := range typ.Types {
			types = f.fuseIntoUnionTypes(types, t)
		}
		return types
	case *super.TypeFusion:
		return f.fuseIntoUnionTypes(types, typ.Type)
	}
	typKind := typ.Kind()
	for i, t := range types {
		switch {
		case t == typ:
			// This is already in the union.
			return types

		case super.TypeUnder(t) == typUnder:
			types[i] = typUnder
			return types
		case typKind != super.PrimitiveKind && typKind == t.Kind():
			typ := f.fuse(t, typ)
			if s, ok := typ.(*super.TypeFusion); ok {
				typ = s.Type
			}
			types[i] = typ
			return types
		}
	}
	return append(types, typ)
}

func (f *Fuser) fusion(typ super.Type) super.Type {
	if !f.complete {
		return typ
	}
	if typ, ok := typ.(*super.TypeFusion); ok {
		return typ
	}
	return f.sctx.LookupTypeFusion(typ)
}

func indexOfField(fields []super.Field, name string) (int, bool) {
	for i, f := range fields {
		if f.Name == name {
			return i, true
		}
	}
	return -1, false
}

// recChanged returns true iff the two record types are different
// enough after fusing that they need to be wrapped in a fusion type.
// As long as all the fields names and optionality are the same, then
// any type differences in the fused type of the child fields will be
// captured by a fusion wrapper somewhere in the descendent type.
func recChanged(a, b *super.TypeRecord) bool {
	if len(a.Fields) != len(b.Fields) {
		return true
	}
	for k, af := range a.Fields {
		bf := b.Fields[k]
		if af.Name != bf.Name || af.Opt != bf.Opt {
			return true
		}
	}
	return false
}
