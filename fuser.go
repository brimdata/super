package super

import (
	"fmt"
	"slices"
)

// Fuser constructs a fused supertype for all the types passed to Fuse.
type Fuser struct {
	sctx     *Context
	complete bool

	typ   Type
	types map[Type]struct{}
}

// XXX this is used by type checker but I think we can use the other one
func NewFuser(sctx *Context, complete bool) *Fuser {
	return &Fuser{sctx: sctx, complete: complete, types: make(map[Type]struct{})}
}

func (f *Fuser) Fuse(t Type) {
	if _, ok := f.types[t]; ok {
		return
	}
	f.types[t] = struct{}{}
	t = f.fuseInternal(t)
	if f.typ == nil {
		f.typ = t
	} else {
		f.typ = f.fuse(f.typ, t)
	}
}

// Type returns the computed supertype.
func (f *Fuser) Type() Type {
	return f.typ
}

func (f *Fuser) fuse(a, b Type) Type {
	if a == b {
		return a
	}
	if typ, ok := a.(*TypeFusion); ok {
		return f.fusion(f.fuse(typ.Type, b))
	}
	if typ, ok := b.(*TypeFusion); ok {
		return f.fusion(f.fuse(a, typ.Type))
	}
	if isAll(a) || isAll(b) {
		return TypeAll
	}
	switch a := a.(type) {
	case *TypeRecord:
		if b, ok := b.(*TypeRecord); ok {
			fields := slices.Clone(a.Fields)
			// First change all fields to optional that are in "a" but not in "b".
			for k, field := range fields {
				if _, ok := indexOfField(b.Fields, field.Name); !ok {
					fields[k].Type = f.makeOption(fields[k].Type)
				}
			}
			// Now fuse all the fields in "b" that are also in "a" and add the fields
			// that are in "b" but not in "a" as they appear in "b".
			for _, field := range b.Fields {
				i, ok := indexOfField(fields, field.Name)
				if ok {
					fields[i].Type = f.fuse(fields[i].Type, field.Type)
				} else {
					typ := f.makeOption(field.Type)
					fields = append(fields, NewField(field.Name, typ))
				}
			}
			fusedRec := f.sctx.MustLookupTypeRecord(fields)
			if recChanged(a, fusedRec) || recChanged(b, fusedRec) {
				return f.fusion(fusedRec)
			}
			return fusedRec
		}
	case *TypeArray:
		if b, ok := b.(*TypeArray); ok {
			return f.fusion(f.sctx.LookupTypeArray(f.fuse(a.Type, b.Type)))
		}
	case *TypeSet:
		if b, ok := b.(*TypeSet); ok {
			return f.fusion(f.sctx.LookupTypeSet(f.fuse(a.Type, b.Type)))
		}
	case *TypeMap:
		if b, ok := b.(*TypeMap); ok {
			keyType := f.fuse(a.KeyType, b.KeyType)
			valType := f.fuse(a.ValType, b.ValType)
			return f.fusion(f.sctx.LookupTypeMap(keyType, valType))
		}
	case *TypeUnion:
		types := f.fuseIntoUnionTypes(nil, a)
		types = f.fuseIntoUnionTypes(types, b)
		if len(types) == 1 {
			return types[0]
		}
		union := f.sctx.MustLookupTypeUnion(Flatten(types))
		return f.fusion(union)
	case *TypeEnum:
		if b, ok := b.(*TypeEnum); ok {
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
	case *TypeError:
		if b, ok := b.(*TypeError); ok {
			return f.fusion(f.sctx.LookupTypeError(f.fuse(a.Type, b.Type)))
		}
	case *TypeNamed:
		if b, ok := b.(*TypeNamed); ok && a.Name == b.Name {
			// if we got here without match a=b above, then there are
			// two different types with the same name, which the type
			// context shouldn't allow.
			f.redefPanic(a)
		}
		// We don't fuse the body of named types as they are unique and
		// a barrier to type fusion.  Instead we fall through here and ,
		// fuse the named type with the other type.
	}
	if _, ok := b.(*TypeUnion); ok {
		return f.fuse(b, a)
	}
	// Neither a nor b can be an anonymous union at this point.
	return f.fusion(f.sctx.MustLookupTypeUnion([]Type{a, b}))
}

func (f *Fuser) makeOption(t Type) Type {
	if fusion, ok := t.(*TypeFusion); ok {
		return f.sctx.LookupTypeFusion(f.makeOption(fusion.Type))
	}
	return f.sctx.Option(t)
}

func isAll(t Type) bool {
	_, ok := t.(*TypeOfAll)
	return ok
}

func (f *Fuser) redefPanic(named *TypeNamed) {
	previous := f.sctx.LookupByName(named.Name)
	panic(fmt.Sprintf("type %s redefined: %#v to %#v", named.Name, previous, named.Type))
}

func (f *Fuser) fuseInternal(typ Type) Type {
	if typ, ok := typ.(*TypeFusion); ok {
		return f.fusion(f.fuseInternal(typ.Type))
	}
	var out Type
	switch typ := typ.(type) {
	case *TypeRecord:
		fields := slices.Clone(typ.Fields)
		for i, field := range fields {
			fields[i].Type = f.fuseInternal(field.Type)
		}
		out = f.sctx.MustLookupTypeRecord(fields)
	case *TypeArray:
		out = f.sctx.LookupTypeArray(f.fuseInternal(typ.Type))
	case *TypeSet:
		out = f.sctx.LookupTypeSet(f.fuseInternal(typ.Type))
	case *TypeMap:
		out = f.sctx.LookupTypeMap(f.fuseInternal(typ.KeyType), f.fuseInternal(typ.ValType))
	case *TypeUnion:
		var types []Type
		for _, t := range typ.Types {
			types = f.fuseIntoUnionTypes(types, f.fuseInternal(t))
		}
		if len(types) == 1 {
			out = types[0]
		} else {
			out = f.sctx.MustLookupTypeUnion(Flatten(types))
		}
	case *TypeEnum:
		return typ
	case *TypeError:
		out = f.sctx.LookupTypeError(f.fuseInternal(typ.Type))
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
func (f *Fuser) fuseIntoUnionTypes(types []Type, typ Type) []Type {
	switch typ := typ.(type) {
	case *TypeNamed:
		return f.addNamed(types, typ)
	case *TypeUnion:
		for _, t := range typ.Types {
			types = f.fuseIntoUnionTypes(types, t)
		}
		return types
	case *TypeFusion:
		return f.fuseIntoUnionTypes(types, typ.Type)
	}
	typKind := typ.Kind()
	for i, t := range types {
		switch {
		case t == typ:
			// This is already in the union.
			return types
		case typKind != PrimitiveKind && typKind == t.Kind() && !IsTypeNamed(t):
			types[i] = noFusion(f.fuse(t, typ))
			return types
		}
	}
	return append(types, noFusion(typ))
}

func (f *Fuser) addNamed(types []Type, named *TypeNamed) []Type {
	for _, t := range types {
		if existingNamed, ok := t.(*TypeNamed); ok && existingNamed.Name == named.Name {
			if existingNamed.Type != named.Type {
				f.redefPanic(named)
			}
			return types
		}
	}
	return append(types, named)
}

func noFusion(typ Type) Type {
	if s, ok := typ.(*TypeFusion); ok {
		return s.Type
	}
	return typ
}

func (f *Fuser) fusion(typ Type) Type {
	if !f.complete {
		return typ
	}
	if typ, ok := typ.(*TypeFusion); ok {
		return typ
	}
	return f.sctx.LookupTypeFusion(typ)
}

func indexOfField(fields []Field, name string) (int, bool) {
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
func recChanged(a, b *TypeRecord) bool {
	if len(a.Fields) != len(b.Fields) {
		return true
	}
	for k, af := range a.Fields {
		bf := b.Fields[k]
		if af.Name != bf.Name || IsOptionType(af.Type) != IsOptionType(bf.Type) {
			return true
		}
	}
	return false
}
