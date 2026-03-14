package op

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

type Count struct {
	parent vio.Puller
	alias  string
	expr   expr.Evaluator
	count  int64
}

func NewCount(sctx *super.Context, parent vio.Puller, alias string, in expr.Evaluator) *Count {
	o := &Count{parent: parent, alias: alias}
	var elems []expr.RecordElem
	if in != nil {
		elems = append(elems, &expr.SpreadElem{Expr: in})
	}
	elems = append(elems, &expr.FieldElem{Name: alias, Expr: evalfunc(o.evalCount)})
	o.expr = expr.NewRecordExpr(sctx, elems)
	return o
}

func (o *Count) Pull(done bool) (vector.Any, error) {
	vec, err := o.parent.Pull(done)
	if vec == nil || err != nil {
		o.count = 0
		return nil, err
	}
	return o.expr.Eval(vec), nil
}

type evalfunc func(vector.Any) vector.Any

func (e evalfunc) Eval(this vector.Any) vector.Any { return e(this) }

func (o *Count) evalCount(in vector.Any) vector.Any {
	counts := make([]int64, in.Len())
	for i := range in.Len() {
		o.count++
		counts[i] = o.count
	}
	return vector.NewInt(super.TypeInt64, counts)
}
