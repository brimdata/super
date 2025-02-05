package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#nameof
type NameOf struct {
	zctx *super.Context
}

func (n *NameOf) Call(args ...vector.Any) vector.Any {
	vec := args[0]
	typ := vec.Type()
	if named, ok := typ.(*super.TypeNamed); ok {
		return vector.NewConst(super.NewString(named.Name), vec.Len(), nil)
	}
	if typ.ID() != super.IDType {
		return vector.NewMissing(n.zctx, vec.Len())
	}
	nulls := vector.NullsOf(vec)
	out := vector.NewStringEmpty(vec.Len(), nulls)
	var errs []uint32
	for i := range vec.Len() {
		b, null := vector.TypeValueValue(vec, i)
		if null {
			out.Append("")
			continue
		}
		var err error
		if typ, err = n.zctx.LookupByValue(b); err != nil {
			panic(err)
		}
		if named, ok := typ.(*super.TypeNamed); ok {
			out.Append(named.Name)
		} else {
			errs = append(errs, i)
		}
	}
	if len(errs) > 0 {
		out.Nulls = vector.NewInverseView(out.Nulls, errs).(*vector.Bool)
		return vector.Combine(out, errs, vector.NewMissing(n.zctx, uint32(len(errs))))
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#typeof
type TypeOf struct {
	zctx *super.Context
}

func (t *TypeOf) Call(args ...vector.Any) vector.Any {
	val := t.zctx.LookupTypeValue(args[0].Type())
	return vector.NewConst(val, args[0].Len(), nil)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#typename
type TypeName struct {
	zctx *super.Context
}

func (t *TypeName) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	if vec.Type() != super.TypeString {
		return vector.NewWrappedError(t.zctx, "typename: argument must be a string", args[0])
	}
	var errs []uint32
	out := vector.NewTypeValueEmpty(0, nil)
	for i := range vec.Len() {
		s, isnull := vector.StringValue(vec, i)
		if isnull {
			if out.Nulls == nil {
				out.Nulls = vector.NewBoolEmpty(vec.Len(), nil)
			}
			out.Nulls.Set(out.Len())
			out.Append(nil)
			continue
		}

		if typ := t.zctx.LookupTypeDef(s); typ == nil {
			errs = append(errs, i)
		} else {
			out.Append(t.zctx.LookupTypeValue(typ).Bytes())
		}
	}
	out.Nulls.SetLen(out.Len())
	if len(errs) > 0 {
		return vector.Combine(out, errs, vector.NewMissing(t.zctx, uint32(len(errs))))
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#error
type Error struct {
	zctx *super.Context
}

func (e *Error) Call(args ...vector.Any) vector.Any {
	vec := args[0]
	return vector.NewError(e.zctx.LookupTypeError(vec.Type()), vec, nil)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#kind
type Kind struct {
	zctx *super.Context
}

func NewKind(zctx *super.Context) *Kind {
	return &Kind{zctx}
}

func (k *Kind) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	if typ := vec.Type(); typ.ID() != super.IDType {
		s := typ.Kind().String()
		return vector.NewConst(super.NewString(s), vec.Len(), nil)
	}
	out := vector.NewStringEmpty(vec.Len(), nil)
	for i, n := uint32(0), vec.Len(); i < n; i++ {
		var s string
		if bytes, null := vector.TypeValueValue(vec, i); !null {
			typ, err := k.zctx.LookupByValue(bytes)
			if err != nil {
				panic(err)
			}
			s = typ.Kind().String()
		}
		out.Append(s)
	}
	return out
}

func (*Kind) RipUnions() bool {
	return false
}
