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
		out = numberToBool(vec.Values(), index)
	case *vector.Uint:
		out = numberToBool(vec.Values(), index)
	case *vector.Float:
		out = numberToBool(vec.Values(), index)
	case *vector.String:
		vvec, errs := stringToBool(vec, index)
		return vvec, errs, true
	default:
		return nil, nil, false
	}
	nulls := vector.NullsOf(vec)
	if index == nil {
		out.SetNulls(nulls)
	} else {
		out.SetNulls(nulls.Pick(index))
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
	bits := bitvec.NewFalse(n)
	var nulls bitvec.Bits
	if !vec.Nulls().IsZero() {
		nulls = bitvec.NewFalse(n)
	}
	var errs []uint32
	var boollen uint32
	nullsIn := vec.Nulls()
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if nullsIn.IsSet(idx) {
			nulls.Set(boollen)
			boollen++
			continue
		}
		b, err := byteconv.ParseBool(vec.Table().Bytes(idx))
		if err != nil {
			errs = append(errs, i)
			continue
		}
		if b {
			bits.Set(boollen)
		}
		boollen++
	}
	bits.Shorten(boollen)
	if !nulls.IsZero() {
		nulls.Shorten(boollen)
	}
	return vector.NewBool(bits, nulls), errs
}
