package compile

import (
	"errors"
	"fmt"

	"github.com/brimsec/zq/ast"
	"github.com/brimsec/zq/expr"
	"github.com/brimsec/zq/reducer"
	"github.com/brimsec/zq/reducer/field"
)

var (
	ErrUnknownField  = errors.New("unknown field")
	ErrFieldRequired = errors.New("field parameter required")
)

type CompiledReducer struct {
	Target         string // The name of the field where results are stored.
	TargetResolver expr.FieldExprResolver
	Instantiate    func() reducer.Interface
}

func Compile(params ast.Reducer) (CompiledReducer, error) {
	var fld expr.FieldExprResolver
	if params.Field != nil {
		var err error
		if fld, err = expr.CompileFieldExpr(params.Field); err != nil {
			return CompiledReducer{}, err
		}
	} else if params.Op != "Count" {
		return CompiledReducer{}, ErrFieldRequired
	}

	var inst func() reducer.Interface
	switch params.Op {
	case "Count":
		inst = func() reducer.Interface {
			return &reducer.Count{Resolver: fld}
		}
	case "First":
		inst = func() reducer.Interface {
			return &reducer.First{Resolver: fld}
		}
	case "Last":
		inst = func() reducer.Interface {
			return &reducer.Last{Resolver: fld}
		}
	case "Avg":
		inst = func() reducer.Interface {
			return &reducer.Avg{Resolver: fld}
		}
	case "CountDistinct":
		inst = func() reducer.Interface {
			return reducer.NewCountDistinct(fld)
		}
	case "Sum", "Min", "Max":
		inst = func() reducer.Interface {
			return &field.FieldReducer{Op: params.Op, Resolver: fld}
		}
	default:
		return CompiledReducer{}, fmt.Errorf("unknown reducer op: %s", params.Op)
	}

	return CompiledReducer{
		Target:         params.Var,
		TargetResolver: expr.CompileFieldAccess(params.Var),
		Instantiate:    inst,
	}, nil
}
