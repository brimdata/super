package expr

//go:generate go run gencomparefuncs.go

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/vector"
)

type Compare struct {
	zctx   *super.Context
	opCode int
	lhs    Evaluator
	rhs    Evaluator
}

func NewCompare(zctx *super.Context, lhs, rhs Evaluator, op string) *Compare {
	return &Compare{zctx, vector.CompareOpFromString(op), lhs, rhs}
}

func (c *Compare) Eval(val vector.Any) vector.Any {
	return vector.Apply(true, c.eval, c.lhs.Eval(val), c.rhs.Eval(val))
}

func (c *Compare) eval(vecs ...vector.Any) vector.Any {
	lhs := vector.Under(vecs[0])
	rhs := vector.Under(vecs[1])
	lhs, rhs, errVal := coerceVals(c.zctx, lhs, rhs)
	if errVal != nil {
		return errVal
	}
	//XXX need to handle overflow (see sam)
	//XXX nulls... for primitives we just do the compare but we need
	// to or the nulls together
	kind := vector.KindOf(lhs)
	if kind != vector.KindOf(rhs) {
		panic("vector kind mismatch after coerce")
	}
	lform, ok := vector.FormOf(lhs)
	if !ok {
		return vector.NewStringError(c.zctx, coerce.ErrIncompatibleTypes.Error(), lhs.Len())
	}
	rform, ok := vector.FormOf(rhs)
	if !ok {
		return vector.NewStringError(c.zctx, coerce.ErrIncompatibleTypes.Error(), lhs.Len())
	}
	f, ok := compareFuncs[vector.FuncCode(c.opCode, kind, lform, rform)]
	if !ok {
		return vector.NewStringError(c.zctx, coerce.ErrIncompatibleTypes.Error(), lhs.Len())
	}
	return f(lhs, rhs)
}
