package coerce

import (
	"math"

	"github.com/superdb/super"
)

func IntBits(typ super.Type) int {
	switch typ.ID() {
	case super.IDInt8:
		return 8
	case super.IDInt16:
		return 16
	case super.IDInt32:
		return 32
	case super.IDInt64, super.IDDuration, super.IDTime:
		return 64
	default:
		panic(typ)
	}
}

func UintBits(typ super.Type) int {
	switch typ.ID() {
	case super.IDUint8:
		return 8
	case super.IDUint16:
		return 16
	case super.IDUint32:
		return 32
	case super.IDUint64:
		return 64
	default:
		panic(typ)
	}
}

func FromIntOverflowCheck(from, to super.Type) (int64, int64, bool) {
	fromid := from.ID()
	if !super.IsSigned(fromid) {
		panic("from: not int")
	}
	switch toid := to.ID(); {
	case super.IsSigned(toid):
		check := fromid > toid
		switch toid {
		case super.IDInt8:
			return math.MinInt8, math.MaxInt8, check
		case super.IDInt16:
			return math.MinInt16, math.MaxInt16, check
		case super.IDInt32:
			return math.MinInt32, math.MaxInt32, check
		default:
			return 0, 0, false
		}
	case super.IsUnsigned(toid):
		// You have to check no matter what b/c of negative
		switch toid {
		case super.IDUint8:
			return 0, math.MaxUint8, true
		case super.IDUint16:
			return 0, math.MaxUint16, true
		case super.IDUint32:
			return 0, math.MaxUint32, true
		default:
			return 0, math.MaxInt64, true
		}
	case super.IsFloat(toid):
		return 0, 0, false
	default:
		panic("to: non-numeric")
	}
}

func FromUintOverflowCheck(from, to super.Type) (uint64, bool) {
	fromid := from.ID()
	if !super.IsUnsigned(fromid) {
		panic("from: not uint")
	}
	switch toid := to.ID(); {
	case super.IsUnsigned(toid):
		check := fromid > toid
		switch toid {
		case super.IDUint8:
			return math.MaxUint8, check
		case super.IDUint16:
			return math.MaxUint16, check
		case super.IDUint32:
			return math.MaxUint32, check
		default:
			return 0, false
		}
	case super.IsSigned(toid):
		switch toid {
		case super.IDInt8:
			return math.MaxInt8, true
		case super.IDInt16:
			return math.MaxInt16, fromid >= super.IDUint16
		case super.IDInt32:
			return math.MaxInt32, fromid >= super.IDUint32
		default:
			return math.MaxInt64, fromid == super.IDUint64
		}
	case super.IsFloat(toid):
		// float has to be checked because < 0
		return math.MaxUint64, true
	default:
		panic("to: non-numeric")
	}
}

func FromFloatOverflowCheck(from, to super.Type) (float64, float64, bool) {
	fromid := from.ID()
	if !super.IsFloat(fromid) {
		panic("from: not float")
	}
	toid := to.ID()
	switch toid {
	case super.IDUint8:
		return 0, math.MaxUint8, true
	case super.IDUint16:
		return 0, math.MaxUint16, true
	case super.IDUint32:
		return 0, math.MaxUint32, true
	case super.IDUint64:
		return 0, math.MaxUint64, true
	case super.IDInt8:
		return math.MinInt8, math.MaxInt8, true
	case super.IDInt16:
		return math.MinInt16, math.MaxInt16, true
	case super.IDInt32:
		return math.MinInt32, math.MaxInt32, true
	case super.IDInt64, super.IDTime, super.IDDuration:
		return math.MinInt64, math.MaxInt64, true
	case super.IDFloat16, super.IDFloat32, super.IDFloat64:
		return 0, 0, false
	default:
		panic("to: non-numeric")
	}
}
