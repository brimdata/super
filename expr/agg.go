package expr

import (
	"errors"

	"github.com/brimsec/zq/expr/agg"
	"github.com/brimsec/zq/zng"
)

var (
	ErrBadValue      = errors.New("bad value")
	ErrFieldRequired = errors.New("field parameter required")
)

type Aggregator struct {
	pattern agg.Pattern
	expr    Evaluator
	where   Evaluator
}

func NewAggregator(op string, expr, where Evaluator) (*Aggregator, error) {
	pattern, err := agg.NewPattern(op)
	if err != nil {
		return nil, err
	}
	if expr == nil {
		// Count is the only that has no argument so we just return
		// true so it counts each value encountered.
		expr = &Literal{zng.True}
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

func (a *Aggregator) Apply(f agg.Function, rec *zng.Record) error {
	if a.filter(rec) {
		return nil
	}
	zv, err := a.expr.Eval(rec)
	if err != nil {
		if err == ErrNoSuchField {
			err = nil
		}
		return err
	}
	return f.Consume(zv)
}

func (a *Aggregator) filter(rec *zng.Record) bool {
	if a.where == nil {
		return false
	}
	zv, err := a.where.Eval(rec)
	if err != nil {
		return true
	}
	return !zng.IsTrue(zv.Bytes)
}
