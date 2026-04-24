package function

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

type Upcast struct {
	sctx *super.Context
}

func NewUpcast(sctx *super.Context) *Upcast {
	return &Upcast{sctx}
}

func (u *Upcast) Call(args []super.Value) super.Value {
	from, to := args[0], args[1]
	if _, ok := to.Type().(*super.TypeOfType); !ok {
		return u.sctx.WrapError("upcast: type argument not a type", to)
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

func (u *Upcast) Cast(from super.Value, to super.Type) (super.Value, bool) {
	var b scode.Builder
	if ok := u.upcast(&b, from.Type(), from.Bytes(), to); ok {
		return super.NewValue(to, b.Bytes().Body()), true
	}
	return super.Value{}, false
}

func (u *Upcast) upcast(b *scode.Builder, typ super.Type, bytes scode.Bytes, to super.Type) bool {
	if f, ok := typ.(*super.TypeFusion); ok {
		typ, bytes = f.DerefFusion(bytes)
	}
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
	case *super.TypeEnum:
		return u.toEnum(b, typ, bytes, to)
	case *super.TypeError:
		return u.toError(b, typ, bytes, to)
	case *super.TypeNamed:
		return u.toNamed(b, typ, bytes, to)
	case *super.TypeFusion:
		return u.toFusion(b, typ, bytes, to)
	default:
		if typ == to {
			b.Append(bytes)
			return true
		}
		return false
	}
}

func (u *Upcast) toRecord(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeRecord) bool {
	typ, bytes = deunion(typ, bytes)
	recType, ok := typ.(*super.TypeRecord)
	if !ok {
		return false
	}
	b.BeginContainer()
	for _, f := range to.Fields {
		elemType, elemBytes, ok := derefAsBytes(recType, bytes, f.Name)
		if !ok {
			// If this field is optional and not present in the input record,
			// code a none value.
			if union, noneTag := super.OptionUnion(f.Type); union != nil {
				super.BeginUnion(b, noneTag)
				b.Append(nil)
				b.EndContainer()
				continue
			}
			// The field isn't here and isn't option so this is an error.
			return false
		}
		if ok := u.upcast(b, elemType, elemBytes, f.Type); !ok {
			return false
		}
	}
	b.EndContainer()
	return true
}

// XXX consolidate this
func derefAsBytes(typ *super.TypeRecord, bytes scode.Bytes, name string) (super.Type, scode.Bytes, bool) {
	n, ok := typ.IndexOfField(name)
	if !ok {
		return nil, nil, false
	}
	var elem scode.Bytes
	for i, it := 0, bytes.Iter(); i <= n; i++ {
		elem = it.Next()
	}
	return typ.Fields[n].Type, elem, true
}

func (u *Upcast) toArray(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeArray) bool {
	if arrayType, ok := typ.(*super.TypeArray); ok {
		return u.toContainer(b, arrayType.Type, bytes, to.Type)
	}
	return false
}

func (u *Upcast) toSet(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeSet) bool {
	if setType, ok := typ.(*super.TypeSet); ok {
		// XXX normalize set contents? can reach into body here blah
		return u.toContainer(b, setType.Type, bytes, to.Type)
	}
	return false
}

func (u *Upcast) toContainer(b *scode.Builder, elemType super.Type, bytes scode.Bytes, toElemType super.Type) bool {
	b.BeginContainer()
	for it := bytes.Iter(); !it.Done(); {
		elemType, bytes := deunion(elemType, it.Next())
		if ok := u.upcast(b, elemType, bytes, toElemType); !ok {
			return false
		}
	}
	b.EndContainer()
	return true
}

func (u *Upcast) toMap(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeMap) bool {
	mapType, ok := typ.(*super.TypeMap)
	if !ok {
		return false
	}
	b.BeginContainer()
	for it := bytes.Iter(); !it.Done(); {
		if ok := u.upcast(b, mapType.KeyType, it.Next(), to.KeyType); !ok {
			return false
		}
		if ok := u.upcast(b, mapType.ValType, it.Next(), to.ValType); !ok {
			return false
		}
	}
	b.EndContainer()
	return true
}

func (u *Upcast) toUnion(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeUnion) bool {
	// Take the value out of the union (if it is union), then look for it
	// in the target union.
	typ, bytes = deunion(typ, bytes)
	tag := UpcastUnionTag(to.Types, typ)
	if tag < 0 {
		return false
	}
	super.BeginUnion(b, tag)
	if ok := u.upcast(b, typ, bytes, to.Types[tag]); !ok {
		return false
	}
	b.EndContainer()
	return true
}

func (u *Upcast) toEnum(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeEnum) bool {
	enumType, ok := typ.(*super.TypeEnum)
	if !ok {
		return false
	}
	symbol, err := enumType.Symbol(int(super.DecodeUint(bytes)))
	if err != nil {
		return false
	}
	i := to.Lookup(symbol)
	if i < 0 {
		return false
	}
	b.Append(super.EncodeUint(uint64(i)))
	return true
}

func (u *Upcast) toFusion(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeFusion) bool {
	b.BeginContainer()
	if to.Type == super.TypeAll {
		b.Append(bytes)
	} else {
		if ok := u.upcast(b, typ, bytes, to.Type); !ok {
			return false
		}
	}
	subType := u.sctx.LookupTypeValue(typ)
	b.Append(subType.Bytes())
	b.EndContainer()
	return true
}

func deunion(typ super.Type, bytes scode.Bytes) (super.Type, scode.Bytes) {
	if union, ok := typ.(*super.TypeUnion); ok {
		return union.Untag(bytes)
	}
	return typ, bytes
}

func UpcastUnionTag(types []super.Type, out super.Type) int {
	if named, ok := out.(*super.TypeNamed); ok {
		return slices.IndexFunc(types, func(t super.Type) bool {
			typ, ok := t.(*super.TypeNamed)
			return ok && named.Name == typ.Name
		})
	}
	k := out.Kind()
	if k == super.PrimitiveKind {
		id := out.ID()
		return slices.IndexFunc(types, func(t super.Type) bool { return !super.IsTypeNamed(t) && t.ID() == id })
	}
	return slices.IndexFunc(types, func(t super.Type) bool { return !super.IsTypeNamed(t) && t.Kind() == k })
}
func (u *Upcast) toError(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeError) bool {
	if errorType, ok := typ.(*super.TypeError); ok {
		return u.upcast(b, errorType.Type, bytes, to.Type)
	}
	return false
}

func (u *Upcast) toNamed(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeNamed) bool {
	if named, ok := typ.(*super.TypeNamed); ok {
		return u.upcast(b, named.Type, bytes, to.Type)
	}
	return false
}
