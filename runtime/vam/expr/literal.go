package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Literal struct {
	val super.Value
}

var _ Evaluator = (*Literal)(nil)

func NewLiteral(val super.Value) *Literal {
	return &Literal{val: val}
}

func (l Literal) Eval(val vector.Any) vector.Any {
	if l.val.IsNull() {
		return vector.NewNull(val.Len())
	}
	return vector.NewConst(l.val, val.Len())
}
