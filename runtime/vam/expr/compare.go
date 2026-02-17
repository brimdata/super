package expr

//go:generate go run gencomparefuncs.go

import (
	"bytes"
	"fmt"

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
	if vec, ok := CheckForNullThenError(vecs); ok {
		return vec
	}
	lhs := vector.Under(vecs[0])
	rhs := vector.Under(vecs[1])
	lhs, rhs, errVal := coerceVals(c.sctx, lhs, rhs)
	if errVal != nil {
		// if incompatible types return false
		return vector.NewConst(super.False, vecs[0].Len())
	}
	//XXX need to handle overflow (see sam)
	kind := lhs.Kind()
	if kind != rhs.Kind() {
		panic("vector kind mismatch after coerce")
	}
	switch kind {
	case vector.KindIP:
		return c.compareIPs(lhs, rhs)
	case vector.KindNet:
		return c.compareNets(lhs, rhs)
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
		return vector.NewConst(super.False, lhs.Len())
	}
	return f(lhs, rhs)
}

func (c *Compare) compareIPs(lhs, rhs vector.Any) vector.Any {
	out := vector.NewFalse(lhs.Len())
	for i := range lhs.Len() {
		l := vector.IPValue(lhs, i)
		r := vector.IPValue(rhs, i)
		if isCompareOpSatisfied(c.opCode, l.Compare(r)) {
			out.Set(i)
		}
	}
	return out
}

func (c *Compare) compareNets(lhs, rhs vector.Any) vector.Any {
	if c.opCode != vector.CompEQ && c.opCode != vector.CompNE {
		s := fmt.Sprintf("type net incompatible with '%s' operator", vector.CompareOpToString(c.opCode))
		return vector.NewStringError(c.sctx, s, lhs.Len())
	}
	out := vector.NewFalse(lhs.Len())
	for i := range lhs.Len() {
		l := vector.NetValue(lhs, i)
		r := vector.NetValue(rhs, i)
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
		return vector.NewConst(super.False, lhs.Len())
	}
	out := vector.NewFalse(lhs.Len())
	for i := range lhs.Len() {
		l := vector.TypeValueValue(lhs, i)
		r := vector.TypeValueValue(rhs, i)
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
	vec := vecs[0]
	k := vec.Kind()
	if k == vector.KindError {
		return vec
	}
	val := super.NewBool(k == vector.KindNull)
	return vector.NewConst(val, vec.Len())
}
