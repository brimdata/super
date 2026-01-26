package expr

//go:generate go run gencomparefuncs.go

import (
	"bytes"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type Compare struct {
	sctx   *super.Context
	opCode int
	lhs    Evaluator
	rhs    Evaluator
}

func NewCompare(sctx *super.Context, op string, lhs, rhs Evaluator) *Compare {
	return &Compare{sctx, vector.CompareOpFromString(op), lhs, rhs}
}

func (c *Compare) Compare(vec0, vec1 vector.Any) vector.Any {
	return c.eval(vec0, vec1)
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
	nulls := bitvec.Or(vector.NullsOf(lhs), vector.NullsOf(rhs))
	lhs, rhs, errVal := coerceVals(c.sctx, lhs, rhs)
	if errVal != nil {
		// if incompatible types return false
		return vector.NewConst(super.False, vecs[0].Len(), nulls)
	}
	//XXX need to handle overflow (see sam)
	kind := lhs.Kind()
	if kind != rhs.Kind() {
		panic("vector kind mismatch after coerce")
	}
	switch kind {
	case vector.KindIP:
		return c.compareIPs(lhs, rhs, nulls)
	case vector.KindNet:
		return c.compareNets(lhs, rhs, nulls)
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
	if !nulls.IsZero() {
		// Having a null true value can cause incorrect results when and'ing
		// and or'ing with other boolean values. Flip true values to false if
		// they are null.
		bits := bitvec.And(FlattenBool(out).Bits, bitvec.Not(nulls))
		out = vector.NewBool(bits, nulls)
	}
	return out
}

func (c *Compare) compareIPs(lhs, rhs vector.Any, nulls bitvec.Bits) vector.Any {
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

func (c *Compare) compareNets(lhs, rhs vector.Any, nulls bitvec.Bits) vector.Any {
	if c.opCode != vector.CompEQ && c.opCode != vector.CompNE {
		s := fmt.Sprintf("type net incompatible with '%s' operator", vector.CompareOpToString(c.opCode))
		return vector.NewStringError(c.sctx, s, lhs.Len())
	}
	out := vector.NewBoolEmpty(lhs.Len(), nulls)
	for i := range lhs.Len() {
		l, null := vector.NetValue(lhs, i)
		if null {
			continue
		}
		r, null := vector.NetValue(rhs, i)
		if null {
			continue
		}
		set := l == r
		if c.opCode == vector.CompNE {
			set = !set
		}
		if set {
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
		return vector.NewConst(super.False, lhs.Len(), bitvec.Zero)
	}
	out := vector.NewFalse(lhs.Len())
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
		return vector.NewConst(super.True, vec.Len(), bitvec.Zero)
	}
	if nulls := vector.NullsOf(vec); !nulls.IsZero() {
		return vector.NewBool(nulls, bitvec.Zero)
	}
	return vector.NewConst(super.False, vec.Len(), bitvec.Zero)
}
