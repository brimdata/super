package groupby

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/zson"
)

type valRow []agg.Function

func newValRow(aggs []*expr.Aggregator) valRow {
	row := make([]agg.Function, 0, len(aggs))
	for _, a := range aggs {
		row = append(row, a.NewFunction())
	}
	return row
}

func (v valRow) apply(zctx *super.Context, ectx expr.Context, aggs []*expr.Aggregator, this super.Value) {
	for k, a := range aggs {
		a.Apply(zctx, ectx, v[k], this)
	}
}

func (v valRow) consumeAsPartial(rec super.Value, exprs []expr.Evaluator, ectx expr.Context) {
	for k, r := range v {
		val := exprs[k].Eval(ectx, rec)
		if val.IsError() {
			panic(fmt.Errorf("consumeAsPartial: read a Zed error: %s", zson.FormatValue(val)))
		}
		//XXX should do soemthing with errors... they could come from
		// a worker over the network?
		if !val.IsError() {
			r.ConsumeAsPartial(val)
		}
	}
}
