package expr

import (
	zed "github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr/agg"
	"github.com/brimdata/super/vector"
)

type Aggregator struct {
	Pattern agg.Pattern
	Expr    Evaluator
	Where   Evaluator
}

func NewAggregator(name string, expr Evaluator, where Evaluator) (*Aggregator, error) {
	pattern, err := agg.NewPattern(name, expr != nil)
	if err != nil {
		return nil, err
	}
	if expr == nil {
		// Count is the only that has no argument so we just return
		// true so it counts each value encountered.
		expr = NewLiteral(zed.True)
	}
	return &Aggregator{
		Pattern: pattern,
		Expr:    expr,
		Where:   where,
	}, nil
}

func (a *Aggregator) Eval(this vector.Any) vector.Any {
	vec := a.Expr.Eval(this)
	if a.Where == nil {
		return vec
	}
	return vector.Apply(true, a.apply, vec, a.Where.Eval(this))
}

func (a *Aggregator) apply(args ...vector.Any) vector.Any {
	vec, where := args[0], args[1]
	var tags []uint32
	// If type is not bool then we want to filter everything.
	if where.Type().ID() == zed.IDBool {
		for slot := uint32(0); slot < where.Len(); slot++ {
			// XXX Feels like we should have a optimzed version of this.
			if vector.BoolValue(where, slot) {
				tags = append(tags, slot)
			}
		}
	}
	return vector.NewView(tags, vec)
}
