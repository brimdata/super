package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Literal struct {
	sctx *super.Context
	val  super.Value
}

var _ Evaluator = (*Literal)(nil)

func NewLiteral(sctx *super.Context, val super.Value) *Literal {
	return &Literal{sctx: sctx, val: val}
}

func (l Literal) Eval(val vector.Any) vector.Any {
	if l.val.IsNull() {
		return vector.NewNull(val.Len())
	}
	return vector.NewConstFromValue(l.sctx, l.val, val.Len())
}
