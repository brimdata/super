package coerce

import (
	"strconv"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/x448/float16"
)

func ToUint(val super.Value, typUint super.Type) (uint64, bool) {
	val = val.Under()
	switch id := val.Type().ID(); {
	case super.IsUnsigned(id):
		v := val.Uint()
		max, check := FromUintOverflowCheck(val.Type(), typUint)
		return v, !check || v <= max
	case super.IsSigned(id):
		v := val.Int()
		min, max, check := FromIntOverflowCheck(val.Type(), typUint)
		return uint64(v), !check || (v >= min && v <= max)
	case super.IsFloat(id):
		v := val.Float()
		min, max, check := FromFloatOverflowCheck(val.Type(), typUint)
		return uint64(v), !check || v >= min && v <= max
	case id == super.IDBool:
		var v uint64
		if val.Bool() {
			v = 1
		}
		return v, true
	case id == super.IDString:
		v, err := strconv.ParseUint(val.AsString(), 10, UintBits(typUint))
		return v, err == nil
	}
	return 0, false
}

func ToInt(val super.Value, typInt super.Type) (int64, bool) {
	val = val.Under()
	switch id := val.Type().ID(); {
	case super.IsUnsigned(id):
		v := val.Uint()
		max, check := FromUintOverflowCheck(val.Type(), typInt)
		return int64(v), !check || v <= max
	case super.IsSigned(id):
		v := val.Int()
		min, max, check := FromIntOverflowCheck(val.Type(), typInt)
		return v, !check || v >= min && v <= max
	case super.IsFloat(id):
		v := val.Float()
		min, max, check := FromFloatOverflowCheck(val.Type(), typInt)
		return int64(v), !check || v >= min && v <= max
	case id == super.IDBool:
		var v int64
		if val.Bool() {
			v = 1
		}
		return v, true
	case id == super.IDString:
		v, err := strconv.ParseInt(val.AsString(), 10, IntBits(typInt))
		return v, err == nil
	}
	return 0, false
}

func ToFloat(val super.Value, typ super.Type) (float64, bool) {
	var v float64
	val = val.Under()
	fromId := val.Type().ID()
	switch {
	case super.IsUnsigned(fromId):
		v = float64(val.Uint())
	case super.IsSigned(fromId):
		v = float64(val.Int())
	case super.IsFloat(fromId):
		v = val.Float()
	case fromId == super.IDBool:
		if val.Bool() {
			v = 1
		}
	case fromId == super.IDString:
		var err error
		if v, err = byteconv.ParseFloat64(val.Bytes()); err != nil {
			return v, false
		}
	}
	switch typ.ID() {
	case super.IDFloat16:
		if fromId != super.IDFloat16 {
			f16 := float16.Fromfloat32(float32(v))
			return float64(f16.Float32()), true
		}
	case super.IDFloat32:
		if fromId != super.IDFloat16 && fromId != super.IDFloat32 {
			return float64(float32(v)), true
		}
	}
	return v, true
}

func ToBool(val super.Value) (bool, bool) {
	val = val.Under()
	if val.IsString() {
		v, err := byteconv.ParseBool(val.Bytes())
		return v, err == nil
	}
	v, ok := ToInt(val, super.TypeInt64)
	return v != 0, ok
}
