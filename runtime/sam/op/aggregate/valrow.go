package aggregate

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/sup"
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
		val := exprs[k].Eval(rec)
		if val.IsError() {
			panic(fmt.Errorf("consumeAsPartial: encountered error: %s", sup.FormatValue(val)))
		}
		//XXX should do soemthing with errors... they could come from
		// a worker over the network?
		if !val.IsError() {
			r.ConsumeAsPartial(val)
		}
	}
}
