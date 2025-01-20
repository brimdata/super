package cast

import (
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/vector"
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
	out.Nulls = vector.NullsOf(vec)
	if index != nil {
		out.Nulls = vector.NewBoolView(out.Nulls, index)
	}
	return out, nil, true
}

func numberToBool[E numeric](s []E, index []uint32) *vector.Bool {
	n := uint32(len(s))
	if index != nil {
		n = uint32(len(index))
	}
	out := vector.NewBoolEmpty(n, nil)
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
	bools := vector.NewBoolEmpty(n, nil)
	if vec.Nulls != nil {
		bools.Nulls = vector.NewBoolEmpty(n, nil)
	}
	var errs []uint32
	var boollen uint32
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if vec.Nulls.Value(idx) {
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
	bools.SetLen(boollen)
	if bools.Nulls != nil {
		bools.Nulls.SetLen(boollen)
	}
	return bools, errs
}
