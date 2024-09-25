package agg

import (
	"fmt"

	zed "github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Func interface {
	Consume(vector.Any)
	Result() zed.Value
}

type Pattern func() Func

func NewPattern(op string, hasarg bool) (Pattern, error) {
	needarg := true
	var pattern Pattern
	switch op {
	case "count":
		needarg = false
		pattern = func() Func {
			return newAggCount()
		}
	// case "any":
	// 	pattern = func() AggFunc {
	// 		return &Any{}
	// 	}
	// case "avg":
	// 	pattern = func() AggFunc {
	// 		return &Avg{}
	// 	}
	// case "dcount":
	// 	pattern = func() AggFunc {
	// 		return NewDCount()
	// 	}
	// case "fuse":
	// 	pattern = func() AggFunc {
	// 		return newFuse()
	// 	}
	// case "sum":
	// 	pattern = func() Func {
	// 		return newAggSum()
	// 	}
	// case "min":
	// 	pattern = func() AggFunc {
	// 		return newMathReducer(anymath.Min)
	// 	}
	// case "max":
	// 	pattern = func() AggFunc {
	// 		return newMathReducer(anymath.Max)
	// 	}
	// case "union":
	// 	panic("TBD")
	// case "collect":
	// 	panic("TBD")
	// case "and":
	// 	pattern = func() AggFunc {
	// 		return &aggAnd{}
	// 	}
	// case "or":
	// 	pattern = func() AggFunc {
	// 		return &aggOr{}
	// 	}
	default:
		return nil, fmt.Errorf("unknown aggregation function: %s", op)
	}
	if needarg && !hasarg {
		return nil, fmt.Errorf("%s: argument required", op)
	}
	return pattern, nil
}

type aggCount struct {
	count uint64
}

func newAggCount() *aggCount {
	return &aggCount{}
}

func (a *aggCount) Consume(vec vector.Any) {
	a.count += uint64(vec.Len())
}

func (a *aggCount) Result() zed.Value {
	return zed.NewUint64(a.count)
}
