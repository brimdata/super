package expr

import (
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr/cast"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type unaryMinus struct {
	sctx *super.Context
	expr Evaluator
}

func NewUnaryMinus(sctx *super.Context, eval Evaluator) Evaluator {
	return &unaryMinus{sctx, eval}
}

func (u *unaryMinus) Eval(this vector.Any) vector.Any {
	return vector.Apply(true, u.eval, u.expr.Eval(this))
}

func (u *unaryMinus) eval(vecs ...vector.Any) vector.Any {
	vec := vector.Under(vecs[0])
	if vec.Len() == 0 {
		return vec
	}
	if _, ok := vec.(*vector.Error); ok {
		return vec
	}
	id := vec.Type().ID()
	if !super.IsNumber(vec.Type().ID()) {
		return vector.NewWrappedError(u.sctx, "type incompatible with unary '-' operator", vecs[0])
	}
	if super.IsUnsigned(id) {
		var typ super.Type
		switch id {
		case super.IDUint8:
			typ = super.TypeInt8
		case super.IDUint16:
			typ = super.TypeInt16
		case super.IDUint32:
			typ = super.TypeInt32
		default:
			typ = super.TypeInt64
		}
		return u.eval(cast.To(u.sctx, vec, typ))
	}
	out, ok := u.convert(vec)
	if !ok {
		// Overflow for int detected, go slow path.
		return u.slowPath(vec)
	}
	return out
}

func (u *unaryMinus) convert(vec vector.Any) (vector.Any, bool) {
	switch vec := vec.(type) {
	case *vector.Const:
		var val super.Value
		if super.IsFloat(vec.Type().ID()) {
			val = super.NewFloat(vec.Type(), -vec.Value().Float())
		} else {
			v := vec.Value().Int()
			if v == minInt(vec.Type()) {
				return nil, false
			}
			val = super.NewInt(vec.Type(), -vec.Value().Int())
		}
		return vector.NewConst(val, vec.Len(), vec.Nulls), true
	case *vector.Dict:
		out, ok := u.convert(vec.Any)
		if !ok {
			return nil, false
		}
		return &vector.Dict{
			Any:    out,
			Index:  vec.Index,
			Counts: vec.Counts,
			Nulls:  vec.Nulls,
		}, true
	case *vector.View:
		out, ok := u.convert(vec.Any)
		if !ok {
			return nil, false
		}
		return &vector.View{Any: out, Index: vec.Index}, true
	case *vector.Int:
		min := minInt(vec.Type())
		out := make([]int64, vec.Len())
		for i := range vec.Len() {
			if vec.Values[i] == min {
				return nil, false
			}
			out[i] = -vec.Values[i]
		}
		return vector.NewInt(vec.Typ, out, vec.Nulls), true
	case *vector.Float:
		out := make([]float64, vec.Len())
		for i := range vec.Len() {
			out[i] = -vec.Values[i]
		}
		return vector.NewFloat(vec.Typ, out, vec.Nulls), true
	default:
		panic(vec)
	}
}

func (u *unaryMinus) slowPath(vec vector.Any) vector.Any {
	var nulls bitvec.Bits
	var ints []int64
	var errs []uint32
	minval := minInt(vec.Type())
	for i := range vec.Len() {
		v, isnull := vector.IntValue(vec, i)
		if isnull {
			if nulls.IsZero() {
				nulls = bitvec.NewFalse(vec.Len())
			}
			nulls.Set(uint32(len(ints)))
			ints = append(ints, 0)
			continue
		}
		if v == minval {
			errs = append(errs, i)
		} else {
			ints = append(ints, -v)
		}
	}
	if !nulls.IsZero() {
		nulls.Shorten(uint32(len(ints)))
	}
	out := vector.NewInt(vec.Type(), ints, nulls)
	err := vector.NewWrappedError(u.sctx, "unary '-' underflow", vector.Pick(vec, errs))
	return vector.Combine(out, errs, err)
}

func minInt(typ super.Type) int64 {
	switch typ.ID() {
	case super.IDInt8:
		return math.MinInt8
	case super.IDInt16:
		return math.MinInt16
	case super.IDInt32:
		return math.MinInt32
	default:
		return math.MinInt64
	}
}
