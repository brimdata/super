package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/agg"
)

type Aggregator struct {
	pattern agg.Pattern
	expr    Evaluator
	where   Evaluator
}

func NewAggregator(op string, distinct bool, expr Evaluator, where Evaluator) (*Aggregator, error) {
	pattern, err := agg.NewPattern(op, distinct, expr != nil)
	if err != nil {
		return nil, err
	}
	if expr == nil {
		// Count is the only that has no argument so we just return
		// true so it counts each value encountered.
		expr = &Literal{super.True}
	}
	return &Aggregator{
		pattern: pattern,
		expr:    expr,
		where:   where,
	}, nil
}

func (a *Aggregator) NewFunction() agg.Function {
	return a.pattern()
}

func (a *Aggregator) Apply(sctx *super.Context, f agg.Function, this super.Value) {
	if a.where != nil {
		if val := EvalBool(sctx, this, a.where); !val.AsBool() {
			// XXX Issue #3401: do something with "where" errors.
			return
		}
	}
	v := a.expr.Eval(this)
	if !v.IsMissing() {
		f.Consume(v)
	}
}
