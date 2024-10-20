package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Float struct {
	Typ    zed.Type
	Values []float64
	Nulls  *Bool
}

var _ Any = (*Float)(nil)

func NewFloat(typ zed.Type, values []float64, nulls *Bool) *Float {
	return &Float{Typ: typ, Values: values, Nulls: nulls}
}

func NewFloatEmpty(typ zed.Type, length uint32, nulls *Bool) *Float {
	return NewFloat(typ, make([]float64, 0, length), nulls)
}

func (f *Float) Append(v float64) {
	f.Values = append(f.Values, v)
}

func (f *Float) Type() zed.Type {
	return f.Typ
}

func (f *Float) Len() uint32 {
	return uint32(len(f.Values))
}

func (f *Float) Value(slot uint32) float64 {
	return f.Values[slot]
}

func (f *Float) Serialize(b *zcode.Builder, slot uint32) {
	if f.Nulls.Value(slot) {
		b.Append(nil)
		return
	}
	switch f.Typ.ID() {
	case zed.IDFloat16:
		b.Append(zed.EncodeFloat16(float32(f.Values[slot])))
	case zed.IDFloat32:
		b.Append(zed.EncodeFloat32(float32(f.Values[slot])))
	case zed.IDFloat64:
		b.Append(zed.EncodeFloat64(f.Values[slot]))
	default:
		panic(f.Typ)
	}
}
