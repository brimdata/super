package agg

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Func interface {
	Consume(vector.Any)
	ConsumeAsPartial(vector.Any)
	Result(*super.Context) super.Value
	ResultAsPartial(*super.Context) super.Value
}

type Pattern func() Func

func NewPattern(op string, hasarg bool) (Pattern, error) {
	needarg := true
	var pattern Pattern
	switch op {
	case "count":
		needarg = false
		pattern = func() Func {
			return &count{}
		}
	case "any":
		pattern = func() Func {
			return NewAny()
		}
	case "avg":
		pattern = func() Func {
			return &avg{}
		}
	// case "dcount":
	// 	pattern = func() AggFunc {
	// 		return NewDCount()
	// 	}
	// case "fuse":
	// 	pattern = func() AggFunc {
	// 		return newFuse()
	// 	}
	case "sum":
		pattern = func() Func {
			return newMathReducer(mathSum)
		}
	case "min":
		pattern = func() Func {
			return newMathReducer(mathMin)
		}
	case "max":
		pattern = func() Func {
			return newMathReducer(mathMax)
		}
	case "union":
		pattern = func() Func {
			return newUnion()
		}
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
