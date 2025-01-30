package function

import (
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr/cast"
	"github.com/brimdata/super/vector"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#abs.md
type Abs struct {
	zctx *super.Context
}

func (a *Abs) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	switch id := vec.Type().ID(); {
	case super.IsUnsigned(id):
		return vec
	case super.IsSigned(id) || super.IsFloat(id):
		return a.abs(vec)
	}
	return vector.NewWrappedError(a.zctx, "abs: not a number", vec)
}

func (a *Abs) abs(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.Const:
		var val super.Value
		if super.IsFloat(vec.Type().ID()) {
			val = super.NewFloat(vec.Type(), math.Abs(vec.Value().Float()))
		} else {
			v := vec.Value().Int()
			if v < 0 {
				v = -v
			}
			val = super.NewInt(vec.Type(), v)
		}
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.View:
		return vector.NewView(a.abs(vec.Any), vec.Index)
	case *vector.Dict:
		return vector.NewDict(a.abs(vec.Any), vec.Index, vec.Counts, vec.Nulls)
	case *vector.Int:
		var ints []int64
		for _, v := range vec.Values {
			if v < 0 {
				v = -v
			}
			ints = append(ints, v)
		}
		return vector.NewInt(vec.Type(), ints, vec.Nulls)
	case *vector.Float:
		var floats []float64
		for _, v := range vec.Values {
			floats = append(floats, math.Abs(v))
		}
		return vector.NewFloat(vec.Type(), floats, vec.Nulls)
	default:
		panic(vec)
	}
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#ceil
type Ceil struct {
	zctx *super.Context
}

func (c *Ceil) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	switch id := vec.Type().ID(); {
	case super.IsFloat(id):
		return c.ceil(vec)
	case super.IsNumber(id):
		return vec
	}
	return vector.NewWrappedError(c.zctx, "ceil: not a number", vec)
}

func (c *Ceil) ceil(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.Const:
		val := super.NewFloat(vec.Type(), math.Ceil(vec.Value().Float()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.View:
		return vector.NewView(c.ceil(vec.Any), vec.Index)
	case *vector.Dict:
		return vector.NewDict(c.ceil(vec.Any), vec.Index, vec.Counts, vec.Nulls)
	case *vector.Float:
		var floats []float64
		for _, v := range vec.Values {
			floats = append(floats, math.Ceil(v))
		}
		return vector.NewFloat(vec.Type(), floats, vec.Nulls)
	default:
		panic(vec)
	}
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#floor
type Floor struct {
	zctx *super.Context
}

func (f *Floor) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	switch id := vec.Type().ID(); {
	case super.IsFloat(id):
		return f.floor(vec)
	case super.IsNumber(id):
		return vec
	}
	return vector.NewWrappedError(f.zctx, "floor: not a number", vec)
}

func (f *Floor) floor(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.Const:
		val := super.NewFloat(vec.Type(), math.Floor(vec.Value().Float()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.View:
		return vector.NewView(f.floor(vec.Any), vec.Index)
	case *vector.Dict:
		return vector.NewDict(f.floor(vec.Any), vec.Index, vec.Counts, vec.Nulls)
	case *vector.Float:
		var floats []float64
		for _, v := range vec.Values {
			floats = append(floats, math.Floor(v))
		}
		return vector.NewFloat(vec.Type(), floats, vec.Nulls)
	default:
		panic(vec)
	}
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#log
type Log struct {
	zctx *super.Context
}

func (l *Log) Call(args ...vector.Any) vector.Any {
	arg := vector.Under(args[0])
	if !super.IsNumber(arg.Type().ID()) {
		if vector.KindOf(arg) == vector.KindError {
			return arg
		}
		return vector.NewWrappedError(l.zctx, "log: not a number", arg)
	}
	// No error casting number to float so no need to Apply.
	vec := cast.To(l.zctx, arg, super.TypeFloat64)
	var errs []uint32
	var floats []float64
	var nulls *vector.Bool
	for i := range vec.Len() {
		v, isnull := vector.FloatValue(vec, i)
		if isnull {
			if nulls == nil {
				nulls = vector.NewBoolEmpty(vec.Len(), nil)
			}
			nulls.Set(uint32(len(floats)))
			floats = append(floats, 0)
			continue
		}
		if v <= 0 {
			errs = append(errs, i)
			continue
		}
		floats = append(floats, math.Log(v))
	}
	out := vector.NewFloat(super.TypeFloat64, floats, nulls)
	if nulls != nil {
		nulls.SetLen(out.Len())
	}
	if len(errs) > 0 {
		err := vector.NewWrappedError(l.zctx, "log: illegal argument", vector.NewView(arg, errs))
		return vector.Combine(out, errs, err)
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#round
type Round struct {
	zctx *super.Context
}

func (r *Round) Call(args ...vector.Any) vector.Any {
	vec := args[0]
	switch id := vec.Type().ID(); {
	case id == super.IDNull:
		return vec
	case super.IsUnsigned(id) || super.IsSigned(id):
		return vec
	case super.IsFloat(id):
		vals := make([]float64, vec.Len())
		for i := range vec.Len() {
			v, _ := vector.FloatValue(vec, i)
			vals[i] = math.Round(v)
		}
		return vector.NewFloat(vec.Type(), vals, vector.NullsOf(vec))
	}
	return vector.NewWrappedError(r.zctx, "round: not a number", vec)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#sqrt
type Sqrt struct {
	zctx *super.Context
}

func (s *Sqrt) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	if id := vec.Type().ID(); id == super.IDNull {
		return vec
	} else if !super.IsNumber(id) {
		return vector.NewWrappedError(s.zctx, "sqrt: number argument required", vec)
	}
	vec = cast.To(s.zctx, vec, super.TypeFloat64)
	vals := make([]float64, vec.Len())
	for i := range vec.Len() {
		v, isnull := vector.FloatValue(vec, i)
		if isnull {
			continue
		}
		vals[i] = math.Sqrt(v)
	}
	return vector.NewFloat(super.TypeFloat64, vals, vector.NullsOf(vec))
}
