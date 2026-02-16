package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr/agg"
	"github.com/brimdata/super/vector"
)

type Aggregator struct {
	Pattern  agg.Pattern
	Name     string
	Distinct bool
	Expr     Evaluator
	Where    Evaluator
}

func NewAggregator(name string, distinct bool, expr Evaluator, where Evaluator) (*Aggregator, error) {
	pattern, err := agg.NewPattern(name, distinct, expr != nil)
	if err != nil {
		return nil, err
	}
	if expr == nil {
		// Count is the only that has no argument so we just return
		// true so it counts each value encountered.
		expr = NewLiteral(super.True)
	}
	return &Aggregator{
		Pattern:  pattern,
		Name:     name,
		Distinct: distinct,
		Expr:     expr,
		Where:    where,
	}, nil
}

func (a *Aggregator) Eval(this vector.Any) vector.Any {
	vec := a.Expr.Eval(this)
	if a.Where == nil {
		return vec
	}
	return vector.Apply(true, a.apply, vec, a.Where.Eval(this))
}

func (*Aggregator) apply(args ...vector.Any) vector.Any {
	vec, where := args[0], args[1]
	bools, _ := BoolMask(where)
	if bools.IsEmpty() {
		// everything is filtered.
		return vector.NewConst(super.Null, vec.Len())
	}
	if bools.GetCardinality() == uint64(vec.Len()) {
		return vec
	}
	index := bools.ToArray()
	nulls := vector.NewConst(super.Null, vec.Len()-uint32(len(index)))
	return vector.Combine(nulls, index, vector.Pick(vec, index))
}
