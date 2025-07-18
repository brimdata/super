package expr

import "github.com/brimdata/super"

type Literal struct {
	val super.Value
}

var _ Evaluator = (*Literal)(nil)

func NewLiteral(val super.Value) *Literal {
	return &Literal{val: val}
}

func (l Literal) Eval(super.Value) super.Value {
	return l.val
}
