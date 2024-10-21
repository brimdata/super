package meta

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
)

type pruner struct {
	pred expr.Evaluator
	ectx expr.Context
}

func newPruner(e expr.Evaluator) *pruner {
	return &pruner{
		pred: e,
		ectx: expr.NewContext(),
	}
}

func (p *pruner) prune(val super.Value) bool {
	if p == nil {
		return false
	}
	result := p.pred.Eval(p.ectx, val)
	return result.Type() == super.TypeBool && result.Bool()
}
