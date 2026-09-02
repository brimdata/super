package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Noneish struct {
	lhs Evaluator
	rhs Evaluator
}

func NewNoneish(sctx *super.Context, lhs, rhs Evaluator) *Noneish {
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
	under := vector.Under(vector.Super(lhs))
	if typ := under.Type(); typ == super.TypeNone || typ == super.TypeNull {
		return rhs
	}
	return lhs
}
