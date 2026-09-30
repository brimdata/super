package expr

import (
	"github.com/superdb/super"
	samexpr "github.com/superdb/super/runtime/sam/expr"
	"github.com/superdb/super/scode"
	"github.com/superdb/super/vector"
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
	vb := vector.NewDynamicValueBuilder()
	for i := range this.Len() {
		val := vector.ValueAt(&s.sb, this, i)
		vb.Write(s.samEval.Eval(val))
	}
	return vb.Build(s.sctx)
}
