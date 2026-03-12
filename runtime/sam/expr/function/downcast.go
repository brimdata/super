package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

type downcast struct {
	sctx    *super.Context
	defuser *defuse
}

func NewDowncast(sctx *super.Context) Caster {
	d, _ := newDowncastDefuser(sctx)
	return d
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
	val, ok := d.Cast(from, typ)
	if !ok {
		return d.sctx.WrapError("downcast: value not a supertype of "+sup.FormatType(typ), from)
	}
	return val
}

func (d *downcast) Cast(from super.Value, to super.Type) (super.Value, bool) {
	var b scode.Builder
	if ok := d.downcast(&b, from.Type(), from.Bytes(), to); ok {
		return super.NewValue(to, b.Bytes().Body()), true
	}
	return super.Value{}, false
}

func (d *downcast) downcast(b *scode.Builder, typ super.Type, bytes scode.Bytes, to super.Type) bool {
	typ, bytes = deunion(typ, bytes)
	if superType, ok := typ.(*super.TypeFusion); ok {
		superBytes, _ := superType.Deref(d.sctx, bytes)
		return d.downcast(b, superType.Type, superBytes, to)
	}
	switch to := to.(type) {
	case *super.TypeRecord:
		return d.toRecord(b, typ, bytes, to)
	case *super.TypeArray:
		return d.toArray(b, typ, bytes, to)
	case *super.TypeSet:
		return d.toSet(b, typ, bytes, to)
	case *super.TypeMap:
		return d.toMap(b, typ, bytes, to)
	case *super.TypeUnion:
		return d.toUnion(b, typ, bytes, to)
	case *super.TypeError:
		return d.toError(b, typ, bytes, to)
	case *super.TypeNamed:
		return d.toNamed(b, typ, bytes, to)
	case *super.TypeFusion:
		// Can't downcast to a super type
		return false
	default:
		if typ == to {
			b.Append(bytes)
			return true
		}
		return false
	}
}

func (d *downcast) toRecord(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeRecord) bool {
	fromType, ok := typ.(*super.TypeRecord)
	if !ok {
		return false
	}
	var nones []int
	var optOff int
	b.BeginContainer()
	for _, toField := range to.Fields { // ranging through to fields and lookup up from
		elemType, elemBytes, none, ok := derefWithNoneAndOk(fromType, bytes, toField.Name)
		if !ok {
			// The super value must have all the fields of the subtype cast.
			// It's missing a field, so fail.
			return false
		}
		if none {
			if !toField.Opt {
				// A none can't go in a non-optional field.
				return false
			}
			nones = append(nones, optOff)
			optOff++
		} else {
			// We have the value and the to field.  Downcast recursively.
			if ok := d.downcast(b, elemType, elemBytes, toField.Type); !ok {
				return false
			}
			if toField.Opt {
				optOff++
			}
		}
	}
	b.EndContainerWithNones(to.Opts, nones)
	return true
}

func (d *downcast) toArray(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeArray) bool {
	if arrayType, ok := typ.(*super.TypeArray); ok {
		return d.toContainer(b, arrayType.Type, bytes, to.Type)
	}
	return false
}

func (d *downcast) toSet(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeSet) bool {
	if setType, ok := typ.(*super.TypeSet); ok {
		// XXX normalize set contents? can reach into body here blah
		return d.toContainer(b, setType.Type, bytes, to.Type)
	}
	return false
}

func (d *downcast) toContainer(b *scode.Builder, typ super.Type, bytes scode.Bytes, to super.Type) bool {
	b.BeginContainer()
	for it := bytes.Iter(); !it.Done(); {
		if ok := d.downcast(b, typ, it.Next(), to); !ok {
			return false
		}
	}
	b.EndContainer()
	return true
}

func (d *downcast) toMap(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeMap) bool {
	mapType, ok := typ.(*super.TypeMap)
	if !ok {
		return false
	}
	b.BeginContainer()
	for it := bytes.Iter(); !it.Done(); {
		if ok := d.downcast(b, mapType.KeyType, it.Next(), to.KeyType); !ok {
			return false
		}
		if ok := d.downcast(b, mapType.ValType, it.Next(), to.ValType); !ok {
			return false
		}
	}
	b.EndContainer()
	return true
}

func (d *downcast) toUnion(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeUnion) bool {
	val := d.defuser.eval(super.NewValue(typ, bytes))
	tag := to.TagOf(val.Type())
	if tag < 0 {
		return false
	}
	super.BuildUnion(b, tag, val.Bytes())
	return true
}

func (d *downcast) toError(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeError) bool {
	if errorType, ok := typ.(*super.TypeError); ok {
		return d.downcast(b, errorType.Type, bytes, to.Type)
	}
	return false
}

func (d *downcast) toNamed(b *scode.Builder, typ super.Type, bytes scode.Bytes, to *super.TypeNamed) bool {
	if namedType, ok := typ.(*super.TypeNamed); ok {
		return d.downcast(b, namedType.Type, bytes, to.Type)
	}
	return false
}
