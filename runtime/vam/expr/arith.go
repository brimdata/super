package expr

//go:generate go run genarithfuncs.go

import (
	"fmt"

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

func (a *Arith) eval(vecs ...vector.Any) vector.Any {
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
	return f(lhs, rhs)
}
