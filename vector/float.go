package vector

import (
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Float struct {
	Typ    super.Type
	Values []float64
	Nulls  *Bool
}

var _ Any = (*Float)(nil)

func NewFloat(typ super.Type, values []float64, nulls *Bool) *Float {
	return &Float{Typ: typ, Values: values, Nulls: nulls}
}

func NewFloatEmpty(typ super.Type, length uint32, nulls *Bool) *Float {
	return NewFloat(typ, make([]float64, 0, length), nulls)
}

func (f *Float) Append(v float64) {
	f.Values = append(f.Values, v)
}

func (f *Float) Type() super.Type {
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
	case super.IDFloat16:
		b.Append(super.EncodeFloat16(float32(f.Values[slot])))
	case super.IDFloat32:
		b.Append(super.EncodeFloat32(float32(f.Values[slot])))
	case super.IDFloat64:
		b.Append(super.EncodeFloat64(f.Values[slot]))
	default:
		panic(f.Typ)
	}
}

func (f *Float) AppendKey(b []byte, slot uint32) []byte {
	if f.Nulls.Value(slot) {
		b = append(b, 0)
	}
	val := math.Float64bits(f.Values[slot])
	b = append(b, byte(val>>(8*7)))
	b = append(b, byte(val>>(8*6)))
	b = append(b, byte(val>>(8*5)))
	b = append(b, byte(val>>(8*4)))
	b = append(b, byte(val>>(8*3)))
	b = append(b, byte(val>>(8*2)))
	b = append(b, byte(val>>(8*1)))
	return append(b, byte(val>>(8*0)))
}
