package cast

import (
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

func castToBool(vec vector.Any, index []uint32) (vector.Any, []uint32, bool) {
	var out *vector.Bool
	switch vec := vec.(type) {
	case *vector.Int:
		out = numberToBool(vec.Values, index)
	case *vector.Uint:
		out = numberToBool(vec.Values, index)
	case *vector.Float:
		out = numberToBool(vec.Values, index)
	case *vector.String:
		vvec, errs := stringToBool(vec, index)
		return vvec, errs, true
	default:
		return nil, nil, false
	}
	nulls := vector.NullsOf(vec)
	if index == nil {
		out.Nulls = nulls
	} else {
		out.Nulls = nulls.Pick(index)
	}
	return out, nil, true
}

func numberToBool[E numeric](s []E, index []uint32) *vector.Bool {
	n := uint32(len(s))
	if index != nil {
		n = uint32(len(index))
	}
	out := vector.NewFalse(n)
	for i := range uint32(len(s)) {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if s[idx] != 0 {
			out.Set(i)
		}
	}
	return out
}

func stringToBool(vec *vector.String, index []uint32) (vector.Any, []uint32) {
	n := lengthOf(vec, index)
	bools := vector.NewFalse(n)
	if !vec.Nulls.IsZero() {
		bools.Nulls = bitvec.NewFalse(n)
	}
	var errs []uint32
	var boollen uint32
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if vec.Nulls.IsSet(idx) {
			bools.Nulls.Set(boollen)
			boollen++
			continue
		}
		bytes := vec.Bytes[vec.Offsets[idx]:vec.Offsets[idx+1]]
		b, err := byteconv.ParseBool(bytes)
		if err != nil {
			errs = append(errs, i)
			continue
		}
		if b {
			bools.Set(boollen)
		}
		boollen++
	}
	bools.Bits.Shorten(boollen)
	if !bools.Nulls.IsZero() {
		bools.Nulls.Shorten(boollen)
	}
	return bools, errs
}
