package aggregate

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/agg"
)

type valRow []agg.Function

func newValRow(aggs []*expr.Aggregator) valRow {
	row := make([]agg.Function, 0, len(aggs))
	for _, a := range aggs {
		row = append(row, a.NewFunction())
	}
	return row
}

func (v valRow) apply(sctx *super.Context, aggs []*expr.Aggregator, this super.Value) {
	for k, a := range aggs {
		a.Apply(sctx, v[k], this)
	}
}

func (v valRow) consumeAsPartial(rec super.Value, exprs []expr.Evaluator) {
	for k, r := range v {
		r.ConsumeAsPartial(exprs[k].Eval(rec))
	}
}
