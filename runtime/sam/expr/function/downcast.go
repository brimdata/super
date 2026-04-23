package function

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

type downcast struct {
	sctx *super.Context
	name string
}

func NewDowncast(sctx *super.Context, name string) Caster {
	return &downcast{sctx, name}
}

func (d *downcast) Call(args []super.Value) super.Value {
	from, to := args[0], args[1]
	if _, ok := super.TypeUnder(to.Type()).(*super.TypeOfType); !ok {
		return d.sctx.WrapError("downcast: type argument not a type", to)
	}
	typ, err := d.sctx.LookupByValue(to.Bytes())
	if err != nil {
		panic(err)
	}
	val, errVal := d.downcast(from.Type(), from.Bytes(), typ)
	if errVal != nil {
		return *errVal
	}
	return val
}

func (d *downcast) Cast(from super.Value, to super.Type) (super.Value, bool) {
	val, errVal := d.downcast(from.Type(), from.Bytes(), to)
	return val, errVal == nil
}

func (d *downcast) downcast(typ super.Type, bytes scode.Bytes, to super.Type) (super.Value, *super.Value) {
	if typ == super.TypeAll {
		return super.NewValue(to, bytes), nil
	}
	if _, ok := to.(*super.TypeUnion); !ok {
		if fusionType, ok := typ.(*super.TypeFusion); ok {
			superBytes, subtype := fusionType.Deref(d.sctx, bytes)
			if subtype != to {
				val, errVal := d.defuse(fusionType, bytes)
				if errVal != nil {
					panic(errVal)
				}
				return super.Value{}, d.errSubtype(val.Type(), val.Bytes(), to)
			}
			return d.downcast(fusionType.Type, superBytes, subtype)
		}
	}
	typ, bytes = deunion(typ, bytes)
	switch to := to.(type) {
	case *super.TypeRecord:
		return d.toRecord(typ, bytes, to)
	case *super.TypeArray:
		return d.toArray(typ, bytes, to)
	case *super.TypeSet:
		return d.toSet(typ, bytes, to)
	case *super.TypeMap:
		return d.toMap(typ, bytes, to)
	case *super.TypeUnion:
		return d.toUnion(typ, bytes, to)
	case *super.TypeError:
		return d.toError(typ, bytes, to)
	case *super.TypeNamed:
		return d.toNamed(typ, bytes, to)
	case *super.TypeFusion:
		// Can't downcast to a super type
		return super.Value{}, d.sctx.WrapError("downcast: cannot downcast to a fusion type", super.NewValue(typ, bytes)).Ptr()
	default:
		if typ == to {
			return super.NewValue(typ, bytes), nil
		} else {
			typ, bytes := deunion(typ, bytes)
			if typ == to {
				return super.NewValue(typ, bytes), nil
			}
		}
		return super.Value{}, d.errMismatch(typ, bytes, to)
	}
}

func (d *downcast) defuse(fusionType *super.TypeFusion, bytes scode.Bytes) (super.Value, *super.Value) {
	superBytes, subtype := fusionType.Deref(d.sctx, bytes)
	return d.downcast(fusionType.Type, superBytes, subtype)
}

func (d *downcast) toRecord(typ super.Type, bytes scode.Bytes, to *super.TypeRecord) (super.Value, *super.Value) {
	fromType, ok := typ.(*super.TypeRecord)
	if !ok {
		return super.Value{}, d.errMismatch(typ, bytes, to)
	}
	var nones []int
	var optOff int
	b := scode.NewBuilder()
	b.BeginContainer()
	for k, toField := range to.Fields { // ranging through to fields and lookup up from
		elemType, elemBytes, none, ok := derefWithNoneAndOk(fromType, bytes, toField.Name)
		if !ok {
			// The super value must have all the fields of the subtype cast.
			// It's missing a field, so fail.
			return super.Value{}, d.errSubtype(typ, bytes, to)
		}
		if none {
			if !toField.Opt {
				// A none can't go in a non-optional field.
				return super.Value{}, d.errSubtype(typ, bytes, to)
			}
			nones = append(nones, optOff)
			optOff++
		} else if toField.Opt && !fromType.Fields[k].Opt {
			return super.Value{}, d.errSubtype(typ, bytes, to)
		} else {
			// We have the value and the to field.  Downcast recursively.
			val, errVal := d.downcast(elemType, elemBytes, toField.Type)
			if errVal != nil {
				return super.Value{}, errVal
			}
			if toField.Opt {
				optOff++
			}
			b.Append(val.Bytes())
		}
	}
	b.EndContainerWithNones(to.Opts, nones)
	return super.NewValue(to, b.Bytes().Body()), nil
}

func (d *downcast) toArray(typ super.Type, bytes scode.Bytes, to *super.TypeArray) (super.Value, *super.Value) {
	if arrayType, ok := typ.(*super.TypeArray); ok {
		return d.toContainer(arrayType.Type, bytes, to, to.Type)
	}
	return super.Value{}, d.errMismatch(typ, bytes, to)
}

func (d *downcast) toSet(typ super.Type, bytes scode.Bytes, to *super.TypeSet) (super.Value, *super.Value) {
	if setType, ok := typ.(*super.TypeSet); ok {
		// XXX normalize set contents? can reach into body here blah
		return d.toContainer(setType.Type, bytes, to, to.Type)
	}
	return super.Value{}, d.errMismatch(typ, bytes, to)
}

func (d *downcast) toContainer(elemType super.Type, bytes scode.Bytes, to super.Type, toElem super.Type) (super.Value, *super.Value) {
	b := scode.NewBuilder()
	b.BeginContainer()
	for it := bytes.Iter(); !it.Done(); {
		val, errVal := d.downcast(elemType, it.Next(), toElem)
		if errVal != nil {
			return super.Value{}, errVal
		}
		b.Append(val.Bytes())
	}
	b.EndContainer()
	return super.NewValue(to, b.Bytes().Body()), nil
}

func (d *downcast) toMap(typ super.Type, bytes scode.Bytes, to *super.TypeMap) (super.Value, *super.Value) {
	mapType, ok := typ.(*super.TypeMap)
	if !ok {
		return super.Value{}, d.errMismatch(typ, bytes, to)
	}
	b := scode.NewBuilder()
	b.BeginContainer()
	for it := bytes.Iter(); !it.Done(); {
		key, errVal := d.downcast(mapType.KeyType, it.Next(), to.KeyType)
		if errVal != nil {
			return super.Value{}, errVal
		}
		b.Append(key.Bytes())
		val, errVal := d.downcast(mapType.ValType, it.Next(), to.ValType)
		if errVal != nil {
			return super.Value{}, errVal
		}
		b.Append(val.Bytes())
	}
	b.EndContainer()
	return super.NewValue(to, b.Bytes().Body()), nil
}

func (d *downcast) toUnion(typ super.Type, bytes scode.Bytes, to *super.TypeUnion) (super.Value, *super.Value) {
	if typ == to {
		return super.NewValue(typ, bytes), nil
	}
	tag, typ, bytes := d.subTypeOf(typ, bytes, to.Types)
	if tag < 0 {
		if _, ok := typ.(*super.TypeUnion); ok {
			typ, bytes = deunion(typ, bytes)
			return d.downcast(typ, bytes, to)
		}
		return super.Value{}, d.errSubtype(typ, bytes, to)
	}
	val, errVal := d.downcast(typ, bytes, to.Types[tag])
	if errVal != nil {
		return super.Value{}, errVal
	}
	b := scode.NewBuilder()
	super.BeginUnion(b, tag)
	b.Append(val.Bytes())
	b.EndContainer()
	return super.NewValue(to, b.Bytes().Body()), nil
}

// subTypeOf finds the tag in the union array types that this value should be
// downcast to.  If the child value is a fusion value, then the type must match
// the subtype of the fusion value.  Otherwise, the child wasn't fused, and by
// definition of a fusion type, one of the union types must exactly match the
// child type.
func (d *downcast) subTypeOf(typ super.Type, bytes scode.Bytes, types []super.Type) (int, super.Type, []byte) {
	if fusionType, ok := typ.(*super.TypeFusion); ok {
		superBytes, subtype := fusionType.Deref(d.sctx, bytes)
		return slices.Index(types, subtype), fusionType.Type, superBytes
	}
	return slices.Index(types, typ), typ, bytes
}

func (d *downcast) toError(typ super.Type, bytes scode.Bytes, to *super.TypeError) (super.Value, *super.Value) {
	if errorType, ok := typ.(*super.TypeError); ok {
		body, errVal := d.downcast(errorType.Type, bytes, to.Type)
		if errVal != nil {
			return super.Value{}, errVal
		}
		return super.NewValue(to, body.Bytes()), nil
	}
	return super.Value{}, d.errMismatch(typ, bytes, to)
}

func (d *downcast) toNamed(typ super.Type, bytes scode.Bytes, to *super.TypeNamed) (super.Value, *super.Value) {
	if unionType, ok := typ.(*super.TypeUnion); ok {
		typ, bytes = deunion(typ, bytes)
		// If we are casting a union type to a named, we need to look through the
		// union for the named type in question since type fusion fuses named
		// types by name.  Then when we find the name, we need to form the subtype
		// from the union options present.
		for _, t := range unionType.Types {
			if named, ok := t.(*super.TypeNamed); ok && named.Name == to.Name {
				typ, bytes = deunion(typ, bytes)
				return super.NewValue(to, bytes), nil
			}
		}
		return super.Value{}, d.errMismatch(typ, bytes, to)
	}
	if fromType, ok := typ.(*super.TypeNamed); ok {
		if fromType.Name != to.Name {
			return super.Value{}, d.errMismatch(typ, bytes, to)
		}
		val, errVal := d.downcast(fromType.Type, bytes, to.Type)
		if errVal != nil {
			return super.Value{}, errVal
		}
		return super.NewValue(to, val.Bytes()), errVal
	}
	val, errVal := d.downcast(typ, bytes, to.Type)
	if errVal != nil {
		return super.Value{}, errVal
	}
	return super.NewValue(to, val.Bytes()), errVal
}

func (d *downcast) errMismatch(typ super.Type, bytes []byte, to super.Type) *super.Value {
	return d.sctx.WrapError("downcast: type mismatch to "+sup.FormatType(to), super.NewValue(typ, bytes)).Ptr()
}

func (d *downcast) errSubtype(typ super.Type, bytes []byte, to super.Type) *super.Value {
	return d.sctx.WrapError("downcast: invalid subtype "+sup.FormatType(to), super.NewValue(typ, bytes)).Ptr()
}
