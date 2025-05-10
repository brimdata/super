package anymath

import "math"

type Float64 func(float64, float64) float64
type Int64 func(int64, int64) int64
type Uint64 func(uint64, uint64) uint64
type String func(string, string) string

type Function struct {
	Init
	Float64
	Int64
	Uint64
	String
}

type Init struct {
	Float64 float64
	Int64   int64
	Uint64  uint64
}

var Min = &Function{
	Init: Init{math.MaxFloat64, math.MaxInt64, math.MaxUint64},
	Float64: func(a, b float64) float64 {
		if a < b {
			return a
		}
		return b
	},
	Int64: func(a, b int64) int64 {
		if a < b {
			return a
		}
		return b
	},
	Uint64: func(a, b uint64) uint64 {
		if a < b {
			return a
		}
		return b
	},
	String: func(a, b string) string {
		if a < b {
			return a
		}
		return b
	},
}

var Max = &Function{
	Init: Init{-math.MaxFloat64, math.MinInt64, 0},
	Float64: func(a, b float64) float64 {
		if a > b {
			return a
		}
		return b
	},
	Int64: func(a, b int64) int64 {
		if a > b {
			return a
		}
		return b
	},
	Uint64: func(a, b uint64) uint64 {
		if a > b {
			return a
		}
		return b
	},
	String: func(a, b string) string {
		if a > b {
			return a
		}
		return b
	},
}

var Add = &Function{
	Float64: func(a, b float64) float64 { return a + b },
	Int64:   func(a, b int64) int64 { return a + b },
	Uint64:  func(a, b uint64) uint64 { return a + b },
}
