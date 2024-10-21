package expr

import "github.com/brimdata/super"

type Var int

var _ Evaluator = (*Var)(nil)

func NewVar(slot int) *Var {
	return (*Var)(&slot)
}

func (v Var) Eval(ectx Context, _ super.Value) super.Value {
	return ectx.Vars()[v]
}
