package expr

//go:generate go run genarithfuncs.go

import (
	"fmt"
	"runtime"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/vector"
)

type Arith struct {
	zctx   *super.Context
	opCode int
	lhs    Evaluator
	rhs    Evaluator
}

func NewArith(zctx *super.Context, lhs, rhs Evaluator, op string) *Arith {
	return &Arith{zctx, vector.ArithOpFromString(op), lhs, rhs}
}

func (a *Arith) Eval(val vector.Any) vector.Any {
	return vector.Apply(true, a.eval, a.lhs.Eval(val), a.rhs.Eval(val))
}

func (a *Arith) eval(vecs ...vector.Any) (out vector.Any) {
	lhs := vector.Under(vecs[0])
	rhs := vector.Under(vecs[1])
	lhs, rhs, errVal := coerceVals(a.zctx, lhs, rhs)
	if errVal != nil {
		return errVal
	}
	kind := vector.KindOf(lhs)
	if kind != vector.KindOf(rhs) {
		panic(fmt.Sprintf("vector kind mismatch after coerce (%#v and %#v)", lhs, rhs))
	}
	if kind == vector.KindFloat && a.opCode == vector.ArithMod {
		return vector.NewStringError(a.zctx, "type float64 incompatible with '%' operator", lhs.Len())
	}
	lform, ok := vector.FormOf(lhs)
	if !ok {
		return vector.NewStringError(a.zctx, coerce.ErrIncompatibleTypes.Error(), lhs.Len())
	}
	rform, ok := vector.FormOf(rhs)
	if !ok {
		return vector.NewStringError(a.zctx, coerce.ErrIncompatibleTypes.Error(), lhs.Len())
	}
	f, ok := arithFuncs[vector.FuncCode(a.opCode, kind, lform, rform)]
	if !ok {
		return vector.NewStringError(a.zctx, coerce.ErrIncompatibleTypes.Error(), lhs.Len())
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

func (a *Arith) evalDivideByZero(kind vector.Kind, lhs, rhs vector.Any) vector.Any {
	var errs []uint32
	var out vector.Any
	switch kind {
	case vector.KindInt:
		var ints []int64
		nulls := vector.NewBoolEmpty(lhs.Len(), nil)
		for i := range lhs.Len() {
			l, lnull := vector.IntValue(lhs, i)
			r, rnull := vector.IntValue(rhs, i)
			if lnull || rnull {
				nulls.Set(i)
				ints = append(ints, 0)
				continue
			}
			if r == 0 {
				errs = append(errs, i)
				continue
			}
			if a.opCode == vector.ArithDiv {
				ints = append(ints, l/r)
			} else {
				ints = append(ints, l%r)
			}
		}
		out = vector.NewInt(super.TypeInt64, ints, nulls)
	case vector.KindUint:
		var uints []uint64
		nulls := vector.NewBoolEmpty(lhs.Len(), nil)
		for i := range lhs.Len() {
			l, lnull := vector.UintValue(lhs, i)
			r, rnull := vector.UintValue(rhs, i)
			if lnull || rnull {
				nulls.Set(i)
				uints = append(uints, 0)
				continue
			}
			if r == 0 {
				errs = append(errs, i)
				continue
			}
			if a.opCode == vector.ArithDiv {
				uints = append(uints, l/r)
			} else {
				uints = append(uints, l%r)
			}
		}
		out = vector.NewUint(super.TypeUint64, uints, nulls)
	default:
		panic(kind)
	}
	if len(errs) > 0 {
		return vector.Combine(out, errs, vector.NewStringError(a.zctx, "divide by zero", uint32(len(errs))))
	}
	return out
}
