package function

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type HasError struct{}

func (h HasError) Call(args ...vector.Any) vector.Any {
	return h.hasError(args[0])
}

func (h HasError) hasError(in vector.Any) vector.Any {
	var index []uint32
	vec := vector.Under(in)
	if view, ok := in.(*vector.View); ok {
		index = view.Index
		vec = view.Any
	}
	switch vec := vec.(type) {
	case *vector.Record:
		var result vector.Any
		for _, f := range vec.Fields {
			if index != nil {
				f = vector.Pick(f, index)
			}
			if result == nil {
				result = h.hasError(f)
				continue
			}
			result = expr.EvalOr(nil, result, h.hasError(f))
		}
		if result == nil {
			return vector.NewFalse(vec.Len())
		}
		return result
	case *vector.Array:
		return listHasError(h.hasError(vec.Values), index, vec.Offsets)
	case *vector.Set:
		return listHasError(h.hasError(vec.Values), index, vec.Offsets)
	case *vector.Map:
		keys := listHasError(h.hasError(vec.Keys), index, vec.Offsets)
		vals := listHasError(h.hasError(vec.Values), index, vec.Offsets)
		return expr.EvalOr(nil, keys, vals)
	default:
		return vector.Apply(true, IsErr{}.Call, in)
	}
}

func listHasError(inner vector.Any, index, offsets []uint32) vector.Any {
	// XXX This is basically the same logic in search.evalForList we should
	// probably centralize this functionality.
	var index2 []uint32
	out := vector.NewFalse(uint32(len(offsets) - 1))
	for i := range out.Len() {
		idx := i
		if index != nil {
			idx = index[i]
		}
		start, end := offsets[idx], offsets[idx+1]
		n := end - start
		if n == 0 {
			continue
		}
		// Reusing index2 across calls here is safe because view does not
		// escape this loop body.
		index2 = slices.Grow(index2[:0], int(n))[:n]
		for k := range n {
			index2[k] = k + start
		}
		view := vector.Pick(inner, index2)
		if expr.FlattenBool(view).Bits.TrueCount() > 0 {
			out.Set(i)
		}
	}
	return out
}

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
		return vector.NewConst(super.NewBool(err == nil && typ == vec.Type()), vec.Len())
	}
	inTyp := vec.Type()
	out := vector.NewFalse(vec.Len())
	for k := range vec.Len() {
		b := vector.TypeValueValue(typeVal, k)
		typ, err := i.sctx.LookupByValue(b)
		if err == nil && typ == inTyp {
			out.Set(k)
		}
	}
	return out
}

type IsErr struct{}

func (IsErr) Call(args ...vector.Any) vector.Any {
	val := super.NewBool(args[0].Kind() == vector.KindError)
	return vector.NewConst(val, args[0].Len())
}

type NameOf struct {
	sctx *super.Context
}

func (n *NameOf) Call(args ...vector.Any) vector.Any {
	vec := args[0]
	typ := vec.Type()
	if named, ok := typ.(*super.TypeNamed); ok {
		return vector.NewConst(super.NewString(named.Name), vec.Len())
	}
	if typ.ID() != super.IDType {
		return vector.NewMissing(n.sctx, vec.Len())
	}
	out := vector.NewStringEmpty(vec.Len())
	var errs []uint32
	for i := range vec.Len() {
		b := vector.TypeValueValue(vec, i)
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
		return vector.Combine(out, errs, vector.NewMissing(n.sctx, uint32(len(errs))))
	}
	return out
}

type TypeOf struct {
	sctx *super.Context
}

func (t *TypeOf) Call(args ...vector.Any) vector.Any {
	val := t.sctx.LookupTypeValue(args[0].Type())
	return vector.NewConst(val, args[0].Len())
}

type TypeName struct {
	sctx *super.Context
}

func (t *TypeName) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	if vec.Type() != super.TypeString {
		return vector.NewWrappedError(t.sctx, "typename: argument must be a string", args[0])
	}
	var errs []uint32
	out := vector.NewTypeValueEmpty(0)
	for i := range vec.Len() {
		s := vector.StringValue(vec, i)
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

type Error struct {
	sctx *super.Context
}

func (e *Error) Call(args ...vector.Any) vector.Any {
	vec := args[0]
	return vector.NewError(e.sctx.LookupTypeError(vec.Type()), vec)
}

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
		return vector.NewConst(super.NewString(s), vec.Len())
	}
	out := vector.NewStringEmpty(vec.Len())
	for i, n := uint32(0), vec.Len(); i < n; i++ {
		bytes := vector.TypeValueValue(vec, i)
		typ, err := k.sctx.LookupByValue(bytes)
		if err != nil {
			panic(err)
		}
		out.Append(typ.Kind().String())
	}
	return out
}

func (*Kind) RipUnions() bool {
	return false
}
