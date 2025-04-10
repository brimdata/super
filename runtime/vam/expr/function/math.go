package function

import (
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr/cast"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#abs.md
type Abs struct {
	sctx *super.Context
}

func (a *Abs) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	switch id := vec.Type().ID(); {
	case super.IsUnsigned(id):
		return vec
	case super.IsSigned(id) || super.IsFloat(id):
		return a.abs(vec)
	}
	return vector.NewWrappedError(a.sctx, "abs: not a number", vec)
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
		return vector.Pick(a.abs(vec.Any), vec.Index)
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
	sctx *super.Context
}

func (c *Ceil) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	switch id := vec.Type().ID(); {
	case super.IsFloat(id):
		return c.ceil(vec)
	case super.IsNumber(id):
		return vec
	}
	return vector.NewWrappedError(c.sctx, "ceil: not a number", vec)
}

func (c *Ceil) ceil(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.Const:
		val := super.NewFloat(vec.Type(), math.Ceil(vec.Value().Float()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.View:
		return vector.Pick(c.ceil(vec.Any), vec.Index)
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
	sctx *super.Context
}

func (f *Floor) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	switch id := vec.Type().ID(); {
	case super.IsFloat(id):
		return f.floor(vec)
	case super.IsNumber(id):
		return vec
	}
	return vector.NewWrappedError(f.sctx, "floor: not a number", vec)
}

func (f *Floor) floor(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.Const:
		val := super.NewFloat(vec.Type(), math.Floor(vec.Value().Float()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.View:
		return vector.Pick(f.floor(vec.Any), vec.Index)
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
	sctx *super.Context
}

func (l *Log) Call(args ...vector.Any) vector.Any {
	arg := vector.Under(args[0])
	if !super.IsNumber(arg.Type().ID()) {
		if vector.KindOf(arg) == vector.KindError {
			return arg
		}
		return vector.NewWrappedError(l.sctx, "log: not a number", arg)
	}
	// No error casting number to float so no need to Apply.
	vec := cast.To(l.sctx, arg, super.TypeFloat64)
	var errs []uint32
	var floats []float64
	var nulls bitvec.Bits
	for i := range vec.Len() {
		v, isnull := vector.FloatValue(vec, i)
		if isnull {
			if nulls.IsZero() {
				nulls = bitvec.NewFalse(vec.Len())
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
	if !nulls.IsZero() {
		nulls.Shorten(out.Len())
	}
	if len(errs) > 0 {
		err := vector.NewWrappedError(l.sctx, "log: illegal argument", vector.Pick(arg, errs))
		return vector.Combine(out, errs, err)
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#pow
type Pow struct {
	sctx *super.Context
}

func (p *Pow) Call(args ...vector.Any) vector.Any {
	a, b := vector.Under(args[0]), vector.Under(args[1])
	if !super.IsNumber(a.Type().ID()) {
		return vector.NewWrappedError(p.sctx, "pow: not a number", args[0])
	}
	if !super.IsNumber(b.Type().ID()) {
		return vector.NewWrappedError(p.sctx, "pow: not a number", args[1])
	}
	a = cast.To(p.sctx, a, super.TypeFloat64)
	b = cast.To(p.sctx, b, super.TypeFloat64)
	nulls := bitvec.Or(vector.NullsOf(a), vector.NullsOf(b))
	vals := make([]float64, a.Len())
	for i := range a.Len() {
		x, null := vector.FloatValue(a, i)
		if null {
			continue
		}
		y, null := vector.FloatValue(b, i)
		if null {
			continue
		}
		vals[i] = math.Pow(x, y)
	}
	return vector.NewFloat(super.TypeFloat64, vals, nulls)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#round
type Round struct {
	sctx *super.Context
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
	return vector.NewWrappedError(r.sctx, "round: not a number", vec)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#sqrt
type Sqrt struct {
	sctx *super.Context
}

func (s *Sqrt) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	if id := vec.Type().ID(); id == super.IDNull {
		return vec
	} else if !super.IsNumber(id) {
		return vector.NewWrappedError(s.sctx, "sqrt: number argument required", vec)
	}
	vec = cast.To(s.sctx, vec, super.TypeFloat64)
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
