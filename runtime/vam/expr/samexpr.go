package expr

import (
	"github.com/brimdata/super"
	samexpr "github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type samExpr struct {
	samEval samexpr.Evaluator
}

func NewSamExpr(sameval samexpr.Evaluator) Evaluator {
	return &samExpr{samEval: sameval}
}

func (s *samExpr) Eval(this vector.Any) vector.Any {
	var typ super.Type
	dynamic, ok := this.(*vector.Dynamic)
	if !ok {
		typ = this.Type()
	}
	var b zcode.Builder
	vb := vector.NewDynamicBuilder()
	for i := range this.Len() {
		b.Truncate()
		this.Serialize(&b, i)
		if dynamic != nil {
			typ = dynamic.TypeOf(i)
		}
		out := s.samEval.Eval(super.NewValue(typ, b.Bytes().Body()))
		vb.Write(out)
	}
	return vb.Build()
}
