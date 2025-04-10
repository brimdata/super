package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#is
type Is struct {
	sctx *super.Context
}

func (i *Is) Call(args ...vector.Any) vector.Any {
	vec := args[0]
	typeVal := args[1]
	if len(args) == 3 {
		vec = args[1]
		typeVal = args[2]
	}
	if typeVal.Type().ID() != super.IDType {
		return vector.NewWrappedError(i.sctx, "is: type value argument expected", typeVal)
	}
	if c, ok := typeVal.(*vector.Const); ok {
		typ, err := i.sctx.LookupByValue(c.Value().Bytes())
		return vector.NewConst(super.NewBool(err == nil && typ == vec.Type()), vec.Len(), bitvec.Zero)
	}
	inTyp := vec.Type()
	out := vector.NewBoolEmpty(vec.Len(), bitvec.Zero)
	for k := range vec.Len() {
		b, _ := vector.TypeValueValue(typeVal, k)
		typ, err := i.sctx.LookupByValue(b)
		if err == nil && typ == inTyp {
			out.Set(k)
		}
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#nameof
type NameOf struct {
	sctx *super.Context
}

func (n *NameOf) Call(args ...vector.Any) vector.Any {
	vec := args[0]
	typ := vec.Type()
	if named, ok := typ.(*super.TypeNamed); ok {
		return vector.NewConst(super.NewString(named.Name), vec.Len(), bitvec.Zero)
	}
	if typ.ID() != super.IDType {
		return vector.NewMissing(n.sctx, vec.Len())
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
		if typ, err = n.sctx.LookupByValue(b); err != nil {
			panic(err)
		}
		if named, ok := typ.(*super.TypeNamed); ok {
			out.Append(named.Name)
		} else {
			errs = append(errs, i)
		}
	}
	if len(errs) > 0 {
		out.Nulls = out.Nulls.ReversePick(errs)
		return vector.Combine(out, errs, vector.NewMissing(n.sctx, uint32(len(errs))))
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#typeof
type TypeOf struct {
	sctx *super.Context
}

func (t *TypeOf) Call(args ...vector.Any) vector.Any {
	val := t.sctx.LookupTypeValue(args[0].Type())
	return vector.NewConst(val, args[0].Len(), bitvec.Zero)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#typename
type TypeName struct {
	sctx *super.Context
}

func (t *TypeName) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	if vec.Type() != super.TypeString {
		return vector.NewWrappedError(t.sctx, "typename: argument must be a string", args[0])
	}
	var errs []uint32
	out := vector.NewTypeValueEmpty(0, bitvec.Zero)
	for i := range vec.Len() {
		s, isnull := vector.StringValue(vec, i)
		if isnull {
			if out.Nulls.IsZero() {
				out.Nulls = bitvec.NewFalse(vec.Len())
			}
			out.Nulls.Set(out.Len())
			out.Append(nil)
			continue
		}

		if typ := t.sctx.LookupTypeDef(s); typ == nil {
			errs = append(errs, i)
		} else {
			out.Append(t.sctx.LookupTypeValue(typ).Bytes())
		}
	}
	if len(errs) > 0 {
		return vector.Combine(out, errs, vector.NewMissing(t.sctx, uint32(len(errs))))
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#error
type Error struct {
	sctx *super.Context
}

func (e *Error) Call(args ...vector.Any) vector.Any {
	vec := args[0]
	return vector.NewError(e.sctx.LookupTypeError(vec.Type()), vec, bitvec.Zero)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#kind
type Kind struct {
	sctx *super.Context
}

func NewKind(sctx *super.Context) *Kind {
	return &Kind{sctx}
}

func (k *Kind) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	if typ := vec.Type(); typ.ID() != super.IDType {
		s := typ.Kind().String()
		return vector.NewConst(super.NewString(s), vec.Len(), bitvec.Zero)
	}
	out := vector.NewStringEmpty(vec.Len(), bitvec.Zero)
	for i, n := uint32(0), vec.Len(); i < n; i++ {
		var s string
		if bytes, null := vector.TypeValueValue(vec, i); !null {
			typ, err := k.sctx.LookupByValue(bytes)
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
