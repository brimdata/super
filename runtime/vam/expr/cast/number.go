package cast

import (
	"strconv"

	"github.com/araddon/dateparse"
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
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
	nulls := vector.NullsOf(vec)
	if index != nil {
		nulls = nulls.Pick(index)
	}
	switch id := typ.ID(); {
	case super.IsSigned(id):
		vals, errs := toNumeric[int64](vec, typ, index)
		if len(errs) > 0 {
			nulls = nulls.Pick(inverseIndex(errs, nulls.Len()))
		}
		return vector.NewInt(typ, vals, nulls), errs, "", true
	case super.IsUnsigned(id):
		vals, errs := toNumeric[uint64](vec, typ, index)
		if len(errs) > 0 {
			nulls = nulls.Pick(inverseIndex(errs, nulls.Len()))
		}
		return vector.NewUint(typ, vals, nulls), errs, "", true
	case super.IsFloat(id):
		vals, errs := toNumeric[float64](vec, typ, index)
		if errs != nil {
			nulls = nulls.Pick(inverseIndex(errs, nulls.Len()))
		}
		return vector.NewFloat(typ, vals, nulls), errs, "", true
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
	var nulls bitvec.Bits
	var ints []int64
	var errs []uint32
	n := lengthOf(vec, index)
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if vec.Nulls.IsSet(idx) {
			if nulls.IsZero() {
				nulls = bitvec.NewFalse(n)
			}
			nulls.Set(uint32(len(ints)))
			ints = append(ints, 0)
			continue
		}
		v, err := strconv.ParseInt(vec.Table().UnsafeString(idx), 10, bits)
		if err != nil {
			errs = append(errs, i)
			continue
		}
		ints = append(ints, v)
	}
	if !nulls.IsZero() {
		nulls.Shorten(uint32(len(ints)))
	}
	return vector.NewInt(typ, ints, nulls), errs
}

func stringToDuration(vec *vector.String, index []uint32) (vector.Any, []uint32) {
	var nulls bitvec.Bits
	var durs []int64
	var errs []uint32
	for i := range lengthOf(vec, index) {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if vec.Nulls.IsSet(idx) {
			if nulls.IsZero() {
				nulls = bitvec.NewFalse(vec.Len())
			}
			nulls.Set(uint32(len(durs)))
			durs = append(durs, 0)
			continue
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
	if !nulls.IsZero() {
		nulls.Shorten(uint32(len(durs)))
	}
	return vector.NewInt(super.TypeDuration, durs, nulls), errs
}

func stringToTime(vec *vector.String, index []uint32) (vector.Any, []uint32) {
	var nulls bitvec.Bits
	var ts []int64
	var errs []uint32
	for i := range lengthOf(vec, index) {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if vec.Nulls.IsSet(idx) {
			if nulls.IsZero() {
				nulls = bitvec.NewFalse(vec.Len())
			}
			nulls.Set(uint32(len(ts)))
			ts = append(ts, 0)
			continue
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
	if !nulls.IsZero() {
		nulls.Shorten(uint32(len(ts)))
	}
	return vector.NewInt(super.TypeTime, ts, nulls), errs
}

func stringToUint(vec *vector.String, typ super.Type, index []uint32) (vector.Any, []uint32) {
	bits := coerce.UintBits(typ)
	var nulls bitvec.Bits
	var ints []uint64
	var errs []uint32
	for i := range lengthOf(vec, index) {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if vec.Nulls.IsSet(idx) {
			if nulls.IsZero() {
				nulls = bitvec.NewFalse(vec.Len())
			}
			nulls.Set(uint32(len(ints)))
			ints = append(ints, 0)
			continue
		}
		v, err := strconv.ParseUint(vec.Table().UnsafeString(idx), 10, bits)
		if err != nil {
			errs = append(errs, i)
			continue
		}
		ints = append(ints, v)
	}
	if !nulls.IsZero() {
		nulls.Shorten(uint32(len(ints)))
	}
	return vector.NewUint(typ, ints, nulls), errs
}

func stringToFloat(vec *vector.String, typ super.Type, index []uint32) (vector.Any, []uint32) {
	var nulls bitvec.Bits
	var floats []float64
	var errs []uint32
	for i := range lengthOf(vec, index) {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if vec.Nulls.IsSet(idx) {
			if nulls.IsZero() {
				nulls = bitvec.NewFalse(vec.Len())
			}
			nulls.Set(uint32(len(floats)))
			floats = append(floats, 0)
			continue
		}
		v, err := byteconv.ParseFloat64(vec.Table().Bytes(idx))
		if err != nil {
			errs = append(errs, i)
			continue
		}
		floats = append(floats, v)
	}
	if !nulls.IsZero() {
		nulls.Shorten(uint32(len(floats)))
	}
	return vector.NewFloat(typ, floats, nulls), errs
}
