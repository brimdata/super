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
	norip    bool
}

func NewAggregator(name string, distinct bool, expr Evaluator, where Evaluator) (*Aggregator, error) {
	pattern, err := agg.NewPattern(name, distinct, expr != nil)
	if err != nil {
		return nil, err
	}
	var norip bool
	if fn, ok := pattern().(interface{ NoRip() bool }); ok {
		norip = fn.NoRip()
	}
	if expr == nil {
		// Count is the only that has no argument so we just return
		// true so it counts each value encountered.
		expr = NewLiteral(nil, super.True)
	}
	return &Aggregator{
		Pattern:  pattern,
		Name:     name,
		Distinct: distinct,
		Expr:     expr,
		Where:    where,
		norip:    norip,
	}, nil
}

func (a *Aggregator) Eval(this vector.Any) vector.Any {
	vec := a.Expr.Eval(this)
	if a.Where == nil {
		if a.norip {
			vec = vector.AddNoRip(vec)
		}
		return vec
	}
	where := a.Where.Eval(this)
	bools, _ := BoolMask(where)
	if bools.IsEmpty() {
		// everything is filtered.
		return vector.NewNull(vec.Len())
	}
	if bools.GetCardinality() != uint64(vec.Len()) {
		index := bools.ToArray()
		nulls := vector.NewNull(vec.Len() - uint32(len(index)))
		vec = vector.Combine(nulls, index, vector.Pick(vec, index))
	}
	if a.norip {
		vec = vector.AddNoRip(vec)
	}
	return vec
}
