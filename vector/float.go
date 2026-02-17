package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Float struct {
	Typ    super.Type
	Values []float64
}

var _ Any = (*Float)(nil)

func NewFloat(typ super.Type, values []float64) *Float {
	return &Float{typ, values}
}

func NewFloatEmpty(typ super.Type, length uint32) *Float {
	return NewFloat(typ, make([]float64, 0, length))
}

func (f *Float) Append(v float64) {
	f.Values = append(f.Values, v)
}

func (*Float) Kind() Kind {
	return KindFloat
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

func (f *Float) Serialize(b *scode.Builder, slot uint32) {
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

func FloatValue(vec Any, slot uint32) float64 {
	switch vec := Under(vec).(type) {
	case *Float:
		return vec.Value(slot)
	case *Const:
		return vec.Value().Ptr().Float()
	case *Dict:
		return FloatValue(vec.Any, uint32(vec.Index[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return FloatValue(vec.Values[tag], vec.ForwardTagMap()[slot])
	case *View:
		return FloatValue(vec.Any, vec.Index[slot])
	}
	panic(vec)
}
