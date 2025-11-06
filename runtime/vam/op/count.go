package op

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type Count struct {
	parent vector.Puller
	alias  string
	expr   expr.Evaluator
	count  uint64
}

func NewCount(sctx *super.Context, parent vector.Puller, alias string, in expr.Evaluator) *Count {
	o := &Count{parent: parent, alias: alias}
	var elems []expr.RecordElem
	elems = append(elems, expr.RecordElem{Name: alias, Expr: evalfunc(o.evalCount)})
	if in != nil {
		elems = append(elems, expr.RecordElem{Expr: in})
	}
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
	counts := make([]uint64, in.Len())
	for i := range in.Len() {
		o.count++
		counts[i] = o.count
	}
	return vector.NewUint(super.TypeUint64, counts, bitvec.Zero)
}
