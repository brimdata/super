package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Float struct {
	loader FloatLoader
	Typ    super.Type
	values []float64
	Nulls  bitvec.Bits
	length uint32
}

var _ Any = (*Float)(nil)

func NewFloat(typ super.Type, values []float64, nulls bitvec.Bits) *Float {
	return &Float{Typ: typ, values: values, Nulls: nulls, length: uint32(len(values))}
}

func NewFloatWithLoader(typ super.Type, length uint32, loader FloatLoader) *Float {
	return &Float{Typ: typ, loader: loader, length: length}
}

func NewFloatEmpty(typ super.Type, length uint32, nulls bitvec.Bits) *Float {
	return NewFloat(typ, make([]float64, 0, length), nulls)
}

func (f *Float) Append(v float64) {
	f.values = append(f.values, v)
	f.length = uint32(len(f.values))
}

func (f *Float) Type() super.Type {
	return f.Typ
}

func (f *Float) Len() uint32 {
	return f.length
}

func (f *Float) Values() []float64 {
	if f.values == nil {
		f.values, f.Nulls = f.loader.Load()
	}
	return f.values
}

func (f *Float) Value(slot uint32) float64 {
	return f.Values()[slot]
}

func (f *Float) Serialize(b *zcode.Builder, slot uint32) {
	if f.Nulls.IsSet(slot) {
		b.Append(nil)
		return
	}
	switch f.Typ.ID() {
	case super.IDFloat16:
		b.Append(super.EncodeFloat16(float32(f.Values()[slot])))
	case super.IDFloat32:
		b.Append(super.EncodeFloat32(float32(f.Values()[slot])))
	case super.IDFloat64:
		b.Append(super.EncodeFloat64(f.Values()[slot]))
	default:
		panic(f.Typ)
	}
}

func FloatValue(vec Any, slot uint32) (float64, bool) {
	switch vec := Under(vec).(type) {
	case *Float:
		return vec.Value(slot), vec.Nulls.IsSet(slot)
	case *Const:
		return vec.Value().Ptr().Float(), vec.Nulls.IsSet(slot)
	case *Dict:
		if vec.Nulls.IsSet(slot) {
			return 0, true
		}
		return FloatValue(vec.Any, uint32(vec.Index[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return FloatValue(vec.Values[tag], vec.TagMap.Forward[slot])
	case *View:
		return FloatValue(vec.Any, vec.Index[slot])
	}
	panic(vec)
}
