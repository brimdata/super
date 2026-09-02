package expr

import (
	"github.com/brimdata/super/vector"
)

type Noneish struct {
	lhs Evaluator
	rhs Evaluator
}

func NewNoneish(lhs, rhs Evaluator) *Noneish {
	return &Noneish{lhs, rhs}
}

func (i *Noneish) Eval(this vector.Any) vector.Any {
	lhs := i.lhs.Eval(this)
	rhs := i.rhs.Eval(this)
	return vector.Apply(vector.ApplyRipUnions, i.eval, lhs, rhs)
}

func (i *Noneish) eval(vecs ...vector.Any) vector.Any {
	lhs := vecs[0]
	rhs := vecs[1]
	if k := vector.Super(lhs).Kind(); k == vector.KindNull || k == vector.KindNone {
		return rhs
	}
	return lhs
}
