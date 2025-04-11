package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/anymath"
	"github.com/brimdata/super/vector"
	"golang.org/x/exp/constraints"
)

var mathSum = &mathFunc{
	Init:        anymath.Add.Init,
	funcFloat64: sum[float64],
	funcInt64:   sum[int64],
	funcUint64:  sum[uint64],
}

var mathMin = &mathFunc{
	Init:        anymath.Min.Init,
	funcFloat64: min[float64],
	funcInt64:   min[int64],
	funcUint64:  min[uint64],
}

var mathMax = &mathFunc{
	Init:        anymath.Max.Init,
	funcFloat64: max[float64],
	funcInt64:   max[int64],
	funcUint64:  max[uint64],
}

type funcFloat64 func(float64, vector.Any) float64
type funcInt64 func(int64, vector.Any) int64
type funcUint64 func(uint64, vector.Any) uint64

type mathFunc struct {
	anymath.Init
	funcFloat64
	funcInt64
	funcUint64
}

type numeric interface {
	constraints.Integer | constraints.Float
}

func sum[T numeric](state T, vec vector.Any) T {
	switch vec := vec.(type) {
	case *vector.Const:
		v := constToNumeric[T](vec)
		return state + v*T(vec.Len()-vec.Nulls().TrueCount())
	case *vector.Dict:
		return sumFlat(state, vec.Any, nil, vec.Counts())
	case *vector.View:
		return sumFlat(state, vec.Any, vec.Index(), nil)
	default:
		return sumFlat(state, vec, nil, nil)
	}
}

func sumFlat[T numeric](state T, vec vector.Any, index []uint32, counts []uint32) T {
	switch vec := vec.(type) {
	case *vector.Uint:
		return sumOf(state, vec.Values(), index, counts)
	case *vector.Int:
		return sumOf(state, vec.Values(), index, counts)
	case *vector.Float:
		return sumOf(state, vec.Values(), index, counts)
	default:
		panic(vec)
	}
}

func sumOf[T numeric, E numeric](state T, vals []E, index []uint32, counts []uint32) T {
	if index != nil {
		for _, idx := range index {
			state += T(vals[idx])
		}
		return state
	}
	if counts != nil {
		for i, count := range counts {
			state += T(vals[i]) * T(count)
		}
		return state
	}
	for _, v := range vals {
		state += T(v)
	}
	return state
}

func min[T numeric](state T, vec vector.Any) T {
	switch vec := vec.(type) {
	case *vector.Const:
		if v := constToNumeric[T](vec); v < state {
			return v
		}
		return state
	case *vector.Dict:
		return minFlat(state, vec.Any, nil)
	case *vector.View:
		return minFlat(state, vec.Any, vec.Index())
	default:
		return minFlat(state, vec, nil)
	}
}

func minFlat[T numeric](state T, vec vector.Any, index []uint32) T {
	switch vec := vec.(type) {
	case *vector.Uint:
		return minOf(state, vec.Values(), index)
	case *vector.Int:
		return minOf(state, vec.Values(), index)
	case *vector.Float:
		return minOf(state, vec.Values(), index)
	default:
		panic(vec)
	}
}

func minOf[T numeric, E numeric](state T, vals []E, index []uint32) T {
	if index != nil {
		for _, idx := range index {
			if v := T(vals[idx]); v < state {
				state = v
			}
		}
		return state
	}
	for _, v := range vals {
		if v := T(v); v < state {
			state = v
		}
	}
	return state
}

func max[T numeric](state T, vec vector.Any) T {
	switch vec := vec.(type) {
	case *vector.Const:
		if v := constToNumeric[T](vec); v > state {
			return v
		}
		return state
	case *vector.Dict:
		return maxFlat(state, vec.Any, nil)
	case *vector.View:
		return maxFlat(state, vec.Any, vec.Index())
	default:
		return maxFlat(state, vec, nil)
	}
}

func maxFlat[T numeric](state T, vec vector.Any, index []uint32) T {
	switch vec := vec.(type) {
	case *vector.Uint:
		return maxOf(state, vec.Values(), index)
	case *vector.Int:
		return maxOf(state, vec.Values(), index)
	case *vector.Float:
		return maxOf(state, vec.Values(), index)
	default:
		panic(vec)
	}
}

func maxOf[T numeric, E numeric](state T, vals []E, index []uint32) T {
	if index != nil {
		for _, idx := range index {
			if v := T(vals[idx]); v > state {
				state = v
			}
		}
		return state
	}
	for _, v := range vals {
		if v := T(v); v > state {
			state = v
		}
	}
	return state
}

func constToNumeric[T numeric](vec *vector.Const) T {
	val := vec.Value()
	switch id := vec.Type().ID(); {
	case super.IsUnsigned(id):
		return T(val.Uint())
	case super.IsSigned(id):
		return T(val.Int())
	default:
		return T(val.Float())
	}
}
