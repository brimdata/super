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

func (a *Aggregator) Apply(zctx *super.Context, ectx Context, f agg.Function, this super.Value) {
	if a.where != nil {
		if val := EvalBool(zctx, ectx, this, a.where); !val.AsBool() {
			// XXX Issue #3401: do something with "where" errors.
			return
		}
	}
	v := a.expr.Eval(ectx, this)
	if !v.IsMissing() {
		f.Consume(v)
	}
}

// NewAggregatorExpr returns an Evaluator from agg. The returned Evaluator
// retains the same functionality of the aggregation only it returns it's
// current state every time a new value is consumed.
func NewAggregatorExpr(zctx *super.Context, agg *Aggregator) *AggregatorExpr {
	return &AggregatorExpr{agg: agg, zctx: zctx}
}

type AggregatorExpr struct {
	agg  *Aggregator
	fn   agg.Function
	zctx *super.Context
}

var _ Evaluator = (*AggregatorExpr)(nil)
var _ Resetter = (*AggregatorExpr)(nil)

func (s *AggregatorExpr) Eval(ectx Context, val super.Value) super.Value {
	if s.fn == nil {
		s.fn = s.agg.NewFunction()
	}
	s.agg.Apply(s.zctx, ectx, s.fn, val)
	return s.fn.Result(s.zctx)
}

func (s *AggregatorExpr) Reset() {
	s.fn = nil
}

type Resetter interface {
	Reset()
}

type Resetters []Resetter

func (rs Resetters) Reset() {
	for _, r := range rs {
		r.Reset()
	}
}
