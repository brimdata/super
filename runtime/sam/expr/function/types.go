package function

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type TypeOf struct {
	sctx *super.Context
}

func (t *TypeOf) Call(args []super.Value) super.Value {
	return t.sctx.LookupTypeValue(args[0].Type())
}

type NameOf struct {
	sctx *super.Context
}

func (n *NameOf) Call(args []super.Value) super.Value {
	typ := args[0].Type()
	if named, ok := typ.(*super.TypeNamed); ok {
		return super.NewString(named.Name)
	}
	if typ.ID() == super.IDType {
		if args[0].IsNull() {
			return super.NullString
		}
		var err error
		if typ, err = n.sctx.LookupByValue(args[0].Bytes()); err != nil {
			panic(err)
		}
		if named, ok := typ.(*super.TypeNamed); ok {
			return super.NewString(named.Name)
		}
	}
	return n.sctx.Missing()
}

type typeName struct {
	sctx *super.Context
}

func (t *typeName) Call(args []super.Value) super.Value {
	if super.TypeUnder(args[0].Type()) != super.TypeString {
		return t.sctx.WrapError("typename: argument must be a string", args[0])
	}
	name := string(args[0].Bytes())
	typ := t.sctx.LookupTypeDef(name)
	if typ == nil {
		return t.sctx.Missing()
	}
	return t.sctx.LookupTypeValue(typ)
}

type Error struct {
	sctx *super.Context
}

func (e *Error) Call(args []super.Value) super.Value {
	return super.NewValue(e.sctx.LookupTypeError(args[0].Type()), args[0].Bytes())
}

type IsErr struct{}

func (*IsErr) Call(args []super.Value) super.Value {
	val := args[0].Under()
	return super.NewBool(val.IsError() && !val.IsNull())
}

type Is struct {
	sctx *super.Context
}

func (i *Is) Call(args []super.Value) super.Value {
	zvSubject := args[0]
	zvTypeVal := args[1]
	var typ super.Type
	var err error
	if zvTypeVal.Type().ID() != super.IDType {
		return i.sctx.WrapError("is: type value argument expected", zvTypeVal)
	}
	typ, err = i.sctx.LookupByValue(zvTypeVal.Bytes())
	return super.NewBool(err == nil && typ == zvSubject.Type())
}

type HasError struct{}

func (h HasError) Call(args []super.Value) super.Value {
	return super.NewBool(h.hasError(args[0].Type(), args[0].Bytes()))
}

func (h HasError) hasError(t super.Type, b scode.Bytes) bool {
	// If a value is null we can skip since an null error is not an error.
	if b == nil {
		return false
	}
	switch typ := super.TypeUnder(t).(type) {
	case *super.TypeRecord:
		it := b.Iter()
		return slices.ContainsFunc(typ.Fields, func(f super.Field) bool {
			return h.hasError(f.Type, it.Next())
		})
	case *super.TypeArray, *super.TypeSet:
		inner := super.InnerType(typ)
		for it := b.Iter(); !it.Done(); {
			if h.hasError(inner, it.Next()) {
				return true
			}
		}
		return false
	case *super.TypeMap:
		for it := b.Iter(); !it.Done(); {
			if h.hasError(typ.KeyType, it.Next()) || h.hasError(typ.ValType, it.Next()) {
				return true
			}
		}
		return false
	case *super.TypeUnion:
		return h.hasError(typ.Untag(b))
	case *super.TypeError:
		return true
	default:
		return false
	}
}

type Quiet struct {
	sctx *super.Context
}

func (q *Quiet) Call(args []super.Value) super.Value {
	val := args[0]
	if val.IsMissing() {
		return q.sctx.Quiet()
	}
	return val
}

type Kind struct {
	sctx *super.Context
}

func (k *Kind) Call(args []super.Value) super.Value {
	val := args[0]
	var typ super.Type
	if _, ok := super.TypeUnder(val.Type()).(*super.TypeOfType); ok {
		var err error
		typ, err = k.sctx.LookupByValue(val.Bytes())
		if err != nil {
			panic(err)
		}
	} else {
		typ = val.Type()
	}
	return super.NewString(typ.Kind().String())
}
