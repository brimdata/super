package agg

import (
	"fmt"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/anymath"
)

// MaxValueSize limits the size of a value produced by an aggregate function
// since sets and arrays could otherwise grow without bound.
var MaxValueSize = 1024 * 1024 * 1024

// A Pattern is a template for creating instances of aggregator functions.
// NewPattern returns a pattern of the type that should be created and
// an instance is created by simply invoking the pattern funtion.
type Pattern func() Function

type Function interface {
	Consume(zed.Value)
	ConsumeAsPartial(zed.Value)
	Result(*zed.Context) zed.Value
	ResultAsPartial(*zed.Context) zed.Value
}

func NewPattern(op string, hasarg bool) (Pattern, error) {
	needarg := true
	var pattern Pattern
	switch op {
	case "count":
		needarg = false
		pattern = func() Function {
			var c Count
			return &c
		}
	case "any":
		pattern = func() Function {
			return &Any{}
		}
	case "avg":
		pattern = func() Function {
			return &Avg{}
		}
	case "dcount":
		pattern = func() Function {
			return NewDCount()
		}
	case "fuse":
		pattern = func() Function {
			return newFuse()
		}
	case "sum":
		pattern = func() Function {
			return newMathReducer(anymath.Add)
		}
	case "collect_map":
		pattern = func() Function {
			return newCollectMap()
		}
	case "min":
		pattern = func() Function {
			return newMathReducer(anymath.Min)
		}
	case "max":
		pattern = func() Function {
			return newMathReducer(anymath.Max)
		}
	case "union":
		pattern = func() Function {
			return newUnion()
		}
	case "collect":
		pattern = func() Function {
			return &Collect{}
		}
	case "and":
		pattern = func() Function {
			return &And{}
		}
	case "or":
		pattern = func() Function {
			return &Or{}
		}
	default:
		return nil, fmt.Errorf("unknown aggregation function: %s", op)
	}
	if needarg && !hasarg {
		return nil, fmt.Errorf("%s: argument required", op)
	}
	return pattern, nil
}
