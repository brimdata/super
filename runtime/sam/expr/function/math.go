package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/anymath"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
)

type reducer struct {
	sctx *super.Context
	name string
	fn   *anymath.Function
}

func NewReducer(sctx *super.Context, name string, fn *anymath.Function) expr.Function {
	return &reducer{sctx, name, fn}
}

func (r *reducer) Call(args []super.Value) super.Value {
	args = underAll(args)
	for _, val := range args {
		if val.IsNull() {
			return super.Null
		}
	}
	for _, val := range args {
		if val.IsError() {
			return val
		}
	}
	for _, val := range args {
		if !super.IsNumber(val.Type().ID()) {
			return r.errNotNumber(val)
		}
	}
	val0 := args[0]
	switch id := val0.Type().ID(); {
	case super.IsUnsigned(id):
		result := val0.Uint()
		for _, val := range args[1:] {
			v, ok := coerce.ToUint(val, super.TypeUint64)
			if !ok {
				return r.errNotNumber(val)
			}
			result = r.fn.Uint64(result, v)
		}
		return super.NewUint64(result)
	case super.IsSigned(id):
		result := val0.Int()
		for _, val := range args[1:] {
			//XXX this is really bad because we silently coerce
			// floats to ints if we hit a float first
			v, ok := coerce.ToInt(val, super.TypeInt64)
			if !ok {
				return r.errNotNumber(val)
			}
			result = r.fn.Int64(result, v)
		}
		return super.NewInt64(result)
	case super.IsFloat(id):
		//XXX this is wrong like math aggregators...
		// need to be more robust and adjust type as new types encountered
		result := val0.Float()
		for _, val := range args[1:] {
			v, ok := coerce.ToFloat(val, super.TypeFloat64)
			if !ok {
				return r.errNotNumber(val)
			}
			result = r.fn.Float64(result, v)
		}
		return super.NewFloat64(result)
	}
	return r.errNotNumber(val0)
}

func (r *reducer) errNotNumber(val super.Value) super.Value {
	return r.sctx.WrapError(r.name+": not a number", val)
}
