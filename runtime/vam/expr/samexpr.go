package expr

import (
	"github.com/brimdata/super"
	samexpr "github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

type samExpr struct {
	sctx    *super.Context
	samEval samexpr.Evaluator
	sb      scode.Builder
}

func NewSamExpr(sctx *super.Context, sameval samexpr.Evaluator) Evaluator {
	return &samExpr{sctx: sctx, samEval: sameval}
}

func (s *samExpr) Eval(this vector.Any) vector.Any {
	vb := vector.NewDynamicBuilder()
	for i := range this.Len() {
		val := vector.ValueAt(&s.sb, this, i)
		vb.Write(s.samEval.Eval(val))
	}
	return vb.Build(s.sctx)
}
