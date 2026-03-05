package count

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sbuf"
)

type Op struct {
	parent sbuf.Puller
	alias  string
	expr   expr.Evaluator
	count  int64
}

func New(sctx *super.Context, parent sbuf.Puller, alias string, in expr.Evaluator) (*Op, error) {
	o := &Op{parent: parent, alias: alias}
	var elems []expr.RecordElem
	if in != nil {
		elems = append(elems, &expr.SpreadElem{Expr: in})
	}
	elems = append(elems, &expr.FieldElem{Name: alias, Expr: evalfunc(o.evalCount)})
	o.expr = expr.NewRecordExpr(sctx, elems)
	return o, nil
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
	batch, err := o.parent.Pull(done)
	if batch == nil || err != nil {
		o.count = 0
		return nil, err
	}
	out := make([]super.Value, 0, len(batch.Values()))
	for _, val := range batch.Values() {
		out = append(out, o.expr.Eval(val).Copy())
	}
	return sbuf.NewBatch(out), nil
}

type evalfunc func(super.Value) super.Value

func (e evalfunc) Eval(this super.Value) super.Value { return e(this) }

func (o *Op) evalCount(_ super.Value) super.Value {
	o.count++
	return super.NewInt64(o.count)
}
