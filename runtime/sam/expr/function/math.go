package function

import (
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/anymath"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#abs.md
type Abs struct {
	zctx *super.Context
}

func (a *Abs) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0].Under()
	switch id := val.Type().ID(); {
	case super.IsUnsigned(id):
		return val
	case super.IsSigned(id):
		x := val.Int()
		if x < 0 {
			x = -x
		}
		return super.NewInt(val.Type(), x)
	case super.IsFloat(id):
		return super.NewFloat(val.Type(), math.Abs(val.Float()))
	}
	return a.zctx.WrapError("abs: not a number", val)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#ceil
type Ceil struct {
	zctx *super.Context
}

func (c *Ceil) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0]
	switch id := val.Type().ID(); {
	case super.IsUnsigned(id) || super.IsSigned(id):
		return val
	case super.IsFloat(id):
		return super.NewFloat(val.Type(), math.Ceil(val.Float()))
	}
	return c.zctx.WrapError("ceil: not a number", val)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#floor
type Floor struct {
	zctx *super.Context
}

func (f *Floor) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0]
	switch id := val.Type().ID(); {
	case super.IsUnsigned(id) || super.IsSigned(id):
		return val
	case super.IsFloat(id):
		return super.NewFloat(val.Type(), math.Floor(val.Float()))
	}
	return f.zctx.WrapError("floor: not a number", val)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#log
type Log struct {
	zctx *super.Context
}

func (l *Log) Call(_ super.Allocator, args []super.Value) super.Value {
	if args[0].IsError() {
		return args[0]
	}
	x, ok := coerce.ToFloat(args[0], super.TypeFloat64)
	if !ok {
		return l.zctx.WrapError("log: not a number", args[0])
	}
	if args[0].IsNull() {
		return super.NullFloat64
	}
	if x <= 0 {
		return l.zctx.WrapError("log: illegal argument", args[0])
	}
	return super.NewFloat64(math.Log(x))
}

type reducer struct {
	zctx *super.Context
	name string
	fn   *anymath.Function
}

func (r *reducer) Call(_ super.Allocator, args []super.Value) super.Value {
	val0 := args[0]
	switch id := val0.Type().ID(); {
	case super.IsUnsigned(id):
		result := val0.Uint()
		for _, val := range args[1:] {
			v, ok := coerce.ToUint(val, super.TypeUint64)
			if !ok {
				return r.zctx.WrapError(r.name+": not a number", val)
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
				return r.zctx.WrapError(r.name+": not a number", val)
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
				return r.zctx.WrapError(r.name+": not a number", val)
			}
			result = r.fn.Float64(result, v)
		}
		return super.NewFloat64(result)
	}
	return r.zctx.WrapError(r.name+": not a number", val0)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#round
type Round struct {
	zctx *super.Context
}

func (r *Round) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0]
	switch id := val.Type().ID(); {
	case id == super.IDNull:
		return val
	case super.IsUnsigned(id) || super.IsSigned(id):
		return val
	case super.IsFloat(id):
		if val.IsNull() {
			return val
		}
		return super.NewFloat(val.Type(), math.Round(val.Float()))
	}
	return r.zctx.WrapError("round: not a number", val)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#pow
type Pow struct {
	zctx *super.Context
}

func (p *Pow) Call(_ super.Allocator, args []super.Value) super.Value {
	a, b := args[0].Under(), args[1].Under()
	if !super.IsNumber(a.Type().ID()) {
		return p.zctx.WrapError("pow: not a number", args[0])
	}
	if !super.IsNumber(b.Type().ID()) {
		return p.zctx.WrapError("pow: not a number", args[1])
	}
	if a.IsNull() || b.IsNull() {
		return super.NullFloat64
	}
	x, _ := coerce.ToFloat(a, super.TypeFloat64)
	y, _ := coerce.ToFloat(b, super.TypeFloat64)
	return super.NewFloat64(math.Pow(x, y))
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#sqrt
type Sqrt struct {
	zctx *super.Context
}

func (s *Sqrt) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0].Under()
	if id := val.Type().ID(); id == super.IDNull {
		return val
	} else if !super.IsNumber(id) {
		return s.zctx.WrapError("sqrt: number argument required", val)
	}
	if val.IsNull() {
		return super.NullFloat64
	}
	x, ok := coerce.ToFloat(val, super.TypeFloat64)
	if !ok {
		return s.zctx.WrapError("sqrt: not a number", val)
	}
	return super.NewFloat64(math.Sqrt(x))
}
