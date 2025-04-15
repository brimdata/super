package coerce

import (
	"bytes"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"golang.org/x/exp/constraints"
)

func Equal(a, b super.Value) bool {
	if a.IsNull() {
		return b.IsNull()
	} else if b.IsNull() {
		// We know a isn't null.
		return false
	}
	switch aid, bid := a.Type().ID(), b.Type().ID(); {
	case !super.IsNumber(aid) || !super.IsNumber(bid):
		if aid != bid {
			return false
		}
		if aid == super.IDNet {
			return super.DecodeNet(a.Bytes()) == super.DecodeNet(b.Bytes())
		}
		return bytes.Equal(a.Bytes(), b.Bytes())
	case super.IsFloat(aid):
		return a.Float() == ToNumeric[float64](b)
	case super.IsFloat(bid):
		return b.Float() == ToNumeric[float64](a)
	case super.IsSigned(aid):
		av := a.Int()
		if super.IsUnsigned(bid) {
			return uint64(av) == b.Uint() && av >= 0
		}
		return av == b.Int()
	case super.IsSigned(bid):
		bv := b.Int()
		if super.IsUnsigned(aid) {
			return uint64(bv) == a.Uint() && bv >= 0
		}
		return bv == a.Int()
	default:
		return a.Uint() == b.Uint()
	}
}

func ToNumeric[T constraints.Integer | constraints.Float](val super.Value) T {
	if val.IsNull() {
		return 0
	}
	val = val.Under()
	switch id := val.Type().ID(); {
	case super.IsUnsigned(id):
		return T(val.Uint())
	case super.IsSigned(id):
		return T(val.Int())
	case super.IsFloat(id):
		return T(val.Float())
	}
	panic(sup.FormatValue(val))
}
