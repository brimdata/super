package coerce

import (
	"errors"
	"math"

	"github.com/brimdata/super"
)

var ErrIncompatibleTypes = errors.New("incompatible types")
var ErrOverflow = errors.New("integer overflow: uint64 value too large for int64")

func Promote(a, b super.Value) (int, error) {
	a, b = a.Under(), b.Under()
	aid, bid := a.Type().ID(), b.Type().ID()
	switch {
	case aid == bid:
		return aid, nil
	case aid == super.IDNull:
		return bid, nil
	case bid == super.IDNull:
		return aid, nil
	case !super.IsNumber(aid) || !super.IsNumber(bid):
		return 0, ErrIncompatibleTypes
	case super.IsFloat(aid):
		if !super.IsFloat(bid) {
			bid = promoteFloat[bid]
		}
	case super.IsFloat(bid):
		if !super.IsFloat(aid) {
			aid = promoteFloat[aid]
		}
	case super.IsSigned(aid):
		if super.IsUnsigned(bid) {
			if b.Uint() > math.MaxInt64 {
				return 0, ErrOverflow
			}
			bid = promoteInt[bid]
		}
	case super.IsSigned(bid):
		if super.IsUnsigned(aid) {
			if a.Uint() > math.MaxInt64 {
				return 0, ErrOverflow
			}
			aid = promoteInt[aid]
		}
	}
	if aid > bid {
		return aid, nil
	}
	return bid, nil
}

var promoteFloat = []int{
	super.IDFloat16,  // IDUint8      = 0
	super.IDFloat16,  // IDUint16     = 1
	super.IDFloat32,  // IDUint32     = 2
	super.IDFloat64,  // IDUint64     = 3
	super.IDFloat128, // IDUint128    = 4
	super.IDFloat256, // IDUint256    = 5
	super.IDFloat16,  // IDInt8       = 6
	super.IDFloat16,  // IDInt16      = 7
	super.IDFloat32,  // IDInt32      = 8
	super.IDFloat64,  // IDInt64      = 9
	super.IDFloat128, // IDInt128     = 10
	super.IDFloat256, // IDInt256     = 11
	super.IDFloat64,  // IDDuration   = 12
	super.IDFloat64,  // IDTime       = 13
	super.IDFloat16,  // IDFloat16    = 14
	super.IDFloat32,  // IDFloat32    = 15
	super.IDFloat64,  // IDFloat64    = 16
	super.IDFloat128, // IDFloat64    = 17
	super.IDFloat256, // IDFloat64    = 18
	super.IDFloat32,  // IDDecimal32  = 19
	super.IDFloat64,  // IDDecimal64  = 20
	super.IDFloat128, // IDDecimal128 = 21
	super.IDFloat256, // IDDecimal256 = 22
}

var promoteInt = []int{
	super.IDInt8,       // IDUint8      = 0
	super.IDInt16,      // IDUint16     = 1
	super.IDInt32,      // IDUint32     = 2
	super.IDInt64,      // IDUint64     = 3
	super.IDInt128,     // IDUint128    = 4
	super.IDInt256,     // IDUint256    = 5
	super.IDInt8,       // IDInt8       = 6
	super.IDInt16,      // IDInt16      = 7
	super.IDInt32,      // IDInt32      = 8
	super.IDInt64,      // IDInt64      = 9
	super.IDInt128,     // IDInt128     = 10
	super.IDInt256,     // IDInt256     = 11
	super.IDInt64,      // IDDuration   = 12
	super.IDInt64,      // IDTime       = 13
	super.IDFloat16,    // IDFloat16    = 14
	super.IDFloat32,    // IDFloat32    = 15
	super.IDFloat64,    // IDFloat64    = 16
	super.IDFloat128,   // IDFloat64    = 17
	super.IDFloat256,   // IDFloat64    = 18
	super.IDDecimal32,  // IDDecimal32  = 19
	super.IDDecimal64,  // IDDecimal64  = 20
	super.IDDecimal128, // IDDecimal128 = 21
	super.IDDecimal256, // IDDecimal256 = 22
}
