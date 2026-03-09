package function

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

type upcast struct {
	sctx *super.Context
}

func NewUpCaster(sctx *super.Context) Caster {
	return &upcast{sctx: sctx}
}

func (u *upcast) Call(args []super.Value) super.Value {
	from, to := args[0], args[1]
	if _, ok := super.TypeUnder(to.Type()).(*super.TypeOfType); !ok {
		return u.sctx.WrapError("upcast type argument not a type", to)
	}
	typ, err := u.sctx.LookupByValue(to.Bytes())
	if err != nil {
		panic(err)
	}
	val, ok := u.Cast(from, typ)
	if !ok {
		return u.sctx.WrapError("upcast: value not a subtype of "+sup.FormatType(typ), from)
	}
	return val
}

func (u *upcast) Cast(from super.Value, to super.Type) (super.Value, bool) {
	var b scode.Builder
	if ok := u.build(&b, from.Type(), from.Bytes(), to); ok {
		return super.NewValue(to, b.Bytes().Body()), true
	}
	return super.Value{}, false
}

func (u *upcast) build(b *scode.Builder, typ super.Type, bytes scode.Bytes, to super.Type) bool {
	switch to := to.(type) {
	case *super.TypeRecord:
		return u.toRecord(b, typ, bytes, to)
	case *super.TypeArray:
		return u.toArray(b, typ, bytes, to)
	case *super.TypeSet:
		return u.toSet(b, typ, bytes, to)
	case *super.TypeMap:
		return u.toMap(b, typ, bytes, to)
	case *super.TypeUnion:
		return u.toUnion(b, typ, bytes, to)
	case *super.TypeError:
		return u.toError(b, typ, bytes, to)
	case *super.TypeNamed:
		return u.toNamed(b, typ, bytes, to)
	default:
		if typ == to {
			b.Append(bytes)
			return true
		}
		return false
	}
}

func (u *upcast) toRecord(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeRecord) bool {
	recType, ok := typ.(*super.TypeRecord)
	if !ok {
		return false
	}
	var nones []int
	var optOff int
	b.BeginContainer()
	for _, f := range to.Fields {
		elemType, elemBytes, none, ok := derefWithNoneAndOk(recType, bytes, f.Name)
		if !ok {
			if f.Opt {
				nones = append(nones, optOff)
				optOff++
			} else {
				// If we don't have a field that is being upcast into, then that
				// field must be optional (because we're a subtype and we don't have it).
				// So error in this case.
				return false
			}
			continue
		}
		if none {
			nones = append(nones, optOff)
			optOff++
			continue
		}
		if ok := u.build(b, elemType, elemBytes, f.Type); !ok {
			return false
		}
		if f.Opt {
			optOff++
		}
	}
	b.EndContainerWithNones(to.Opts, nones)
	return true
}

func derefWithNoneAndOk(typ *super.TypeRecord, bytes scode.Bytes, name string) (super.Type, scode.Bytes, bool, bool) {
	n, ok := typ.IndexOfField(name)
	if !ok {
		return nil, nil, false, false
	}
	var elem scode.Bytes
	var none bool
	for i, it := 0, scode.NewRecordIter(bytes, typ.Opts); i <= n; i++ {
		elem, none = it.Next(typ.Fields[i].Opt)
	}
	fieldType := typ.Fields[n].Type
	if none {
		return fieldType, nil, true, true
	}
	return fieldType, elem, false, true

}

func (u *upcast) toArray(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeArray) bool {
	if arrayType, ok := typ.(*super.TypeArray); ok {
		return u.toContainer(b, arrayType.Type, bytes, to.Type)
	}
	return false
}

func (u *upcast) toSet(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeSet) bool {
	if setType, ok := typ.(*super.TypeSet); ok {
		// XXX normalize set contents? can reach into body here blah
		return u.toContainer(b, setType.Type, bytes, to.Type)
	}
	return false
}

func (u *upcast) toContainer(b *scode.Builder, typ super.Type, bytes scode.Bytes, to super.Type) bool {
	b.BeginContainer()
	for it := bytes.Iter(); !it.Done(); {
		typ, bytes := deunion(typ, it.Next())
		if ok := u.build(b, typ, bytes, to); !ok {
			return false
		}
	}
	b.EndContainer()
	return true
}

func (u *upcast) toMap(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeMap) bool {
	mapType, ok := typ.(*super.TypeMap)
	if !ok {
		return false
	}
	b.BeginContainer()
	for it := bytes.Iter(); !it.Done(); {
		if ok := u.build(b, mapType.KeyType, it.Next(), to.KeyType); !ok {
			return false
		}
		if ok := u.build(b, mapType.ValType, it.Next(), to.ValType); !ok {
			return false
		}
	}
	b.EndContainer()
	return true
}

func (u *upcast) toUnion(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeUnion) bool {
	// Take the value out of the union (if it is union), then look for it
	// in the target union.
	typ, bytes = deunion(typ, bytes)
	tag := upcastUnionTag(to.Types, typ)
	if tag < 0 {
		return false
	}
	super.BeginUnion(b, tag)
	if ok := u.build(b, typ, bytes, to.Types[tag]); !ok {
		return false
	}
	b.EndContainer()
	return true
}

func deunion(typ super.Type, bytes scode.Bytes) (super.Type, scode.Bytes) {
	if union, ok := typ.(*super.TypeUnion); ok {
		return union.Untag(bytes)
	}
	return typ, bytes
}

func upcastUnionTag(types []super.Type, out super.Type) int {
	k := out.Kind()
	if k == super.PrimitiveKind {
		id := out.ID()
		return slices.IndexFunc(types, func(t super.Type) bool { return t.ID() == id })
	}
	return slices.IndexFunc(types, func(t super.Type) bool { return t.Kind() == k })
}

func (u *upcast) toError(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeError) bool {
	if errorType, ok := typ.(*super.TypeError); ok {
		return u.build(b, errorType.Type, bytes, to.Type)
	}
	return false
}

func (u *upcast) toNamed(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeNamed) bool {
	if namedType, ok := typ.(*super.TypeNamed); ok {
		return u.build(b, namedType.Type, bytes, to.Type)
	}
	return false
}
