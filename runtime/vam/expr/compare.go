package expr

//go:generate go run gencomparefuncs.go

import (
	"bytes"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/vector"
)

type Compare struct {
	sctx   *super.Context
	opCode int
	lhs    Evaluator
	rhs    Evaluator
}

func NewCompare(sctx *super.Context, lhs, rhs Evaluator, op string) *Compare {
	return &Compare{sctx, vector.CompareOpFromString(op), lhs, rhs}
}

func (c *Compare) Eval(val vector.Any) vector.Any {
	return vector.Apply(true, c.eval, c.lhs.Eval(val), c.rhs.Eval(val))
}

func (c *Compare) eval(vecs ...vector.Any) vector.Any {
	lhs := vector.Under(vecs[0])
	rhs := vector.Under(vecs[1])
	if _, ok := lhs.(*vector.Error); ok {
		return vecs[0]
	}
	if _, ok := rhs.(*vector.Error); ok {
		return vecs[1]
	}
	nulls := vector.Or(vector.NullsOf(lhs), vector.NullsOf(rhs))
	lhs, rhs, errVal := coerceVals(c.sctx, lhs, rhs)
	if errVal != nil {
		// if incompatible types return false
		return vector.NewConst(super.False, vecs[0].Len(), nulls)
	}
	//XXX need to handle overflow (see sam)
	kind := vector.KindOf(lhs)
	if kind != vector.KindOf(rhs) {
		panic("vector kind mismatch after coerce")
	}
	switch kind {
	case vector.KindIP:
		return c.compareIPs(lhs, rhs, nulls)
	case vector.KindType:
		return c.compareTypeVals(lhs, rhs)
	}
	lform, ok := vector.FormOf(lhs)
	if !ok {
		return vector.NewStringError(c.sctx, coerce.ErrIncompatibleTypes.Error(), lhs.Len())
	}
	rform, ok := vector.FormOf(rhs)
	if !ok {
		return vector.NewStringError(c.sctx, coerce.ErrIncompatibleTypes.Error(), lhs.Len())
	}
	f, ok := compareFuncs[vector.FuncCode(c.opCode, kind, lform, rform)]
	if !ok {
		return vector.NewConst(super.False, lhs.Len(), nulls)
	}
	out := f(lhs, rhs)
	return vector.CopyAndSetNulls(out, nulls)
}

func (c *Compare) compareIPs(lhs, rhs vector.Any, nulls *vector.Bool) vector.Any {
	out := vector.NewBoolEmpty(lhs.Len(), nulls)
	for i := range lhs.Len() {
		l, null := vector.IPValue(lhs, i)
		if null {
			continue
		}
		r, null := vector.IPValue(rhs, i)
		if null {
			continue
		}
		if isCompareOpSatisfied(c.opCode, l.Compare(r)) {
			out.Set(i)
		}
	}
	return out
}

func isCompareOpSatisfied(opCode, i int) bool {
	switch opCode {
	case vector.CompLT:
		return i < 0
	case vector.CompLE:
		return i <= 0
	case vector.CompGT:
		return i > 0
	case vector.CompGE:
		return i >= 0
	case vector.CompEQ:
		return i == 0
	case vector.CompNE:
		return i != 0
	}
	panic(opCode)
}

func (c *Compare) compareTypeVals(lhs, rhs vector.Any) vector.Any {
	if c.opCode == vector.CompLT || c.opCode == vector.CompGT {
		return vector.NewConst(super.False, lhs.Len(), nil)
	}
	out := vector.NewBoolEmpty(lhs.Len(), nil)
	for i := range lhs.Len() {
		l, _ := vector.TypeValueValue(lhs, i)
		r, _ := vector.TypeValueValue(rhs, i)
		v := bytes.Equal(l, r)
		if c.opCode == vector.CompNE {
			v = !v
		}
		if v {
			out.Set(i)
		}
	}
	return out
}

type isNull struct {
	expr Evaluator
}

func NewIsNull(e Evaluator) Evaluator {
	return &isNull{e}
}

func (i *isNull) Eval(this vector.Any) vector.Any {
	return vector.Apply(false, i.eval, i.expr.Eval(this))
}

func (i *isNull) eval(vecs ...vector.Any) vector.Any {
	vec := vector.Under(vecs[0])
	if _, ok := vec.(*vector.Error); ok {
		return vec
	}
	if c, ok := vec.(*vector.Const); ok && c.Value().IsNull() {
		return vector.NewConst(super.True, vec.Len(), nil)
	}
	if nulls := vector.NullsOf(vec); nulls != nil {
		return nulls
	}
	return vector.NewConst(super.False, vec.Len(), nil)
}
