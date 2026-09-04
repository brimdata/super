package coerce

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"golang.org/x/exp/constraints"
)

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
