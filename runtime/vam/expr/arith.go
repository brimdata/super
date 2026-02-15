package expr

//go:generate go run genarithfuncs.go

import (
	"fmt"
	"runtime"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
)

type Arith struct {
	sctx   *super.Context
	opCode int
	lhs    Evaluator
	rhs    Evaluator
}

func NewArith(sctx *super.Context, op string, lhs, rhs Evaluator) *Arith {
	return &Arith{sctx, vector.ArithOpFromString(op), lhs, rhs}
}

func (a *Arith) Eval(val vector.Any) vector.Any {
	return vector.Apply(true, a.eval, a.lhs.Eval(val), a.rhs.Eval(val))
}

func (a *Arith) eval(vecs ...vector.Any) (out vector.Any) {
	lhs, rhs := vecs[0], vecs[1]
	if k := lhs.Kind(); k == vector.KindNull || k == vector.KindError {
		return lhs
	}
	if k := rhs.Kind(); k == vector.KindNull || k == vector.KindError {
		return rhs
	}
	lhs = enumToIndex(vector.Under(lhs))
	rhs = enumToIndex(vector.Under(rhs))
	lhs, rhs, errVal := coerceVals(a.sctx, lhs, rhs)
	if errVal != nil {
		return errVal
	}
	kind := lhs.Kind()
	if kind != rhs.Kind() {
		panic(fmt.Sprintf("vector kind mismatch after coerce (%#v and %#v)", lhs, rhs))
	}
	if kind == vector.KindFloat && a.opCode == vector.ArithMod {
		return vector.NewStringError(a.sctx, "type float64 incompatible with '%' operator", lhs.Len())
	}
	lform, ok := vector.FormOf(lhs)
	if !ok {
		return vector.NewStringError(a.sctx, coerce.ErrIncompatibleTypes.Error(), lhs.Len())
	}
	rform, ok := vector.FormOf(rhs)
	if !ok {
		return vector.NewStringError(a.sctx, coerce.ErrIncompatibleTypes.Error(), lhs.Len())
	}
	f, ok := arithFuncs[vector.FuncCode(a.opCode, kind, lform, rform)]
	if !ok {
		s := fmt.Sprintf("type %s incompatible with '%s' operator", sup.FormatType(lhs.Type()), vector.ArithOpToString(a.opCode))
		return vector.NewStringError(a.sctx, s, lhs.Len())
	}
	if a.opCode == vector.ArithDiv || a.opCode == vector.ArithMod {
		defer func() {
			if v := recover(); v != nil {
				if err, ok := v.(runtime.Error); ok && err.Error() == "runtime error: integer divide by zero" {
					out = a.evalDivideByZero(kind, lhs, rhs)
				} else {
					panic(v)
				}
			}
		}()
	}
	return f(lhs, rhs)
}

func enumToIndex(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.View:
		if enum, ok := vec.Any.(*vector.Enum); ok {
			return vector.Pick(enum.Uint, vec.Index)
		}
	case *vector.Enum:
		return vec.Uint
	}
	return vec
}

func (a *Arith) evalDivideByZero(kind vector.Kind, lhs, rhs vector.Any) vector.Any {
	var errs []uint32
	var out vector.Any
	switch kind {
	case vector.KindInt:
		var ints []int64
		for i := range lhs.Len() {
			r := vector.IntValue(rhs, i)
			if r == 0 {
				errs = append(errs, i)
				continue
			}
			l := vector.IntValue(lhs, i)
			if a.opCode == vector.ArithDiv {
				ints = append(ints, l/r)
			} else {
				ints = append(ints, l%r)
			}
		}
		out = vector.NewInt(super.TypeInt64, ints)
	case vector.KindUint:
		var uints []uint64
		for i := range lhs.Len() {
			r := vector.UintValue(rhs, i)
			if r == 0 {
				errs = append(errs, i)
				continue
			}
			l := vector.UintValue(lhs, i)
			if a.opCode == vector.ArithDiv {
				uints = append(uints, l/r)
			} else {
				uints = append(uints, l%r)
			}
		}
		out = vector.NewUint(super.TypeUint64, uints)
	default:
		panic(kind)
	}
	if len(errs) > 0 {
		return vector.Combine(out, errs, vector.NewStringError(a.sctx, "divide by zero", uint32(len(errs))))
	}
	return out
}
