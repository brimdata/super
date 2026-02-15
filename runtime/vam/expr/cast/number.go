package cast

import (
	"strconv"

	"github.com/araddon/dateparse"
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/vector"
	"golang.org/x/exp/constraints"
)

type numeric interface {
	constraints.Float | constraints.Integer
}

func castToNumber(vec vector.Any, typ super.Type, index []uint32) (vector.Any, []uint32, string, bool) {
	if vec.Type().ID() == super.IDString {
		out, errs := castStringToNumber(vec, typ, index)
		return out, errs, "", true
	}
	switch id := typ.ID(); {
	case super.IsSigned(id):
		vals, errs := toNumeric[int64](vec, typ, index)
		return vector.NewInt(typ, vals), errs, "", true
	case super.IsUnsigned(id):
		vals, errs := toNumeric[uint64](vec, typ, index)
		return vector.NewUint(typ, vals), errs, "", true
	case super.IsFloat(id):
		vals, errs := toNumeric[float64](vec, typ, index)
		return vector.NewFloat(typ, vals), errs, "", true
	default:
		return nil, nil, "", false
	}
}

func inverseIndex(index []uint32, n uint32) []uint32 {
	var inverse []uint32
	for i := range n {
		if len(index) > 0 && index[0] == i {
			index = index[1:]
			continue
		}
		inverse = append(inverse, i)
	}
	return inverse
}

func toNumeric[T numeric](vec vector.Any, typ super.Type, index []uint32) ([]T, []uint32) {
	switch vec := vec.(type) {
	case *vector.Uint:
		if max, check := coerce.FromUintOverflowCheck(vec.Type(), typ); check {
			return checkAndCastNumbers[uint64, T](vec.Values, 0, max, index)
		}
		return castNumbers[uint64, T](vec.Values, index), nil
	case *vector.Int:
		if min, max, check := coerce.FromIntOverflowCheck(vec.Type(), typ); check {
			return checkAndCastNumbers[int64, T](vec.Values, min, max, index)
		}
		return castNumbers[int64, T](vec.Values, index), nil
	case *vector.Float:
		if min, max, check := coerce.FromFloatOverflowCheck(vec.Type(), typ); check {
			return checkAndCastNumbers[float64, T](vec.Values, min, max, index)
		}
		return castNumbers[float64, T](vec.Values, index), nil
	case *vector.Bool:
		return boolToNumeric[T](vec, index), nil
	default:
		panic(vec)
	}
}

func checkAndCastNumbers[E numeric, T numeric](s []E, min, max E, index []uint32) ([]T, []uint32) {
	var errs []uint32
	var out []T
	if index != nil {
		for i, idx := range index {
			v := s[idx]
			if v < min || v > max {
				errs = append(errs, uint32(i))
				continue
			}
			out = append(out, T(v))
		}
	} else {
		for i, v := range s {
			if v < min || v > max {
				errs = append(errs, uint32(i))
				continue
			}
			out = append(out, T(v))
		}
	}
	return out, errs
}

func castNumbers[E numeric, T numeric](s []E, index []uint32) []T {
	if index != nil {
		out := make([]T, len(index))
		for i, idx := range index {
			out[i] = T(s[idx])
		}
		return out
	}
	out := make([]T, len(s))
	for i, v := range s {
		out[i] = T(v)
	}
	return out
}

func boolToNumeric[T numeric](vec *vector.Bool, index []uint32) []T {
	n := lengthOf(vec, index)
	out := make([]T, n)
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if vec.Bits.IsSet(idx) {
			out[i] = 1
		}
	}
	return out
}

func castStringToNumber(vec vector.Any, typ super.Type, index []uint32) (vector.Any, []uint32) {
	svec := vec.(*vector.String)
	switch id := typ.ID(); {
	case super.IsSigned(id):
		if id == super.IDDuration {
			return stringToDuration(svec, index)
		}
		if id == super.IDTime {
			return stringToTime(svec, index)
		}
		return stringToInt(svec, typ, index)
	case super.IsUnsigned(id):
		return stringToUint(svec, typ, index)
	case super.IsFloat(id):
		return stringToFloat(svec, typ, index)
	default:
		panic(typ)
	}
}

func stringToInt(vec *vector.String, typ super.Type, index []uint32) (vector.Any, []uint32) {
	bits := coerce.IntBits(typ)
	var ints []int64
	var errs []uint32
	n := lengthOf(vec, index)
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		v, err := strconv.ParseInt(vec.Table().UnsafeString(idx), 10, bits)
		if err != nil {
			errs = append(errs, i)
			continue
		}
		ints = append(ints, v)
	}
	return vector.NewInt(typ, ints), errs
}

func stringToDuration(vec *vector.String, index []uint32) (vector.Any, []uint32) {
	var durs []int64
	var errs []uint32
	for i := range lengthOf(vec, index) {
		idx := i
		if index != nil {
			idx = index[i]
		}
		b := vec.Table().Bytes(idx)
		d, err := nano.ParseDuration(byteconv.UnsafeString(b))
		if err != nil {
			f, ferr := byteconv.ParseFloat64(b)
			if ferr != nil {
				errs = append(errs, i)
				continue
			}
			d = nano.Duration(f)
		}
		durs = append(durs, int64(d))
	}
	return vector.NewInt(super.TypeDuration, durs), errs
}

func stringToTime(vec *vector.String, index []uint32) (vector.Any, []uint32) {
	var ts []int64
	var errs []uint32
	for i := range lengthOf(vec, index) {
		idx := i
		if index != nil {
			idx = index[i]
		}
		b := vec.Table().Bytes(idx)
		if gotime, err := dateparse.ParseAny(byteconv.UnsafeString(b)); err != nil {
			f, ferr := byteconv.ParseFloat64(b)
			if ferr != nil {
				errs = append(errs, i)
				continue
			}
			ts = append(ts, int64(f))
		} else {
			ts = append(ts, gotime.UnixNano())
		}
	}
	return vector.NewInt(super.TypeTime, ts), errs
}

func stringToUint(vec *vector.String, typ super.Type, index []uint32) (vector.Any, []uint32) {
	bits := coerce.UintBits(typ)
	var ints []uint64
	var errs []uint32
	for i := range lengthOf(vec, index) {
		idx := i
		if index != nil {
			idx = index[i]
		}
		v, err := strconv.ParseUint(vec.Table().UnsafeString(idx), 10, bits)
		if err != nil {
			errs = append(errs, i)
			continue
		}
		ints = append(ints, v)
	}
	return vector.NewUint(typ, ints), errs
}

func stringToFloat(vec *vector.String, typ super.Type, index []uint32) (vector.Any, []uint32) {
	var floats []float64
	var errs []uint32
	for i := range lengthOf(vec, index) {
		idx := i
		if index != nil {
			idx = index[i]
		}
		v, err := byteconv.ParseFloat64(vec.Table().Bytes(idx))
		if err != nil {
			errs = append(errs, i)
			continue
		}
		floats = append(floats, v)
	}
	return vector.NewFloat(typ, floats), errs
}
