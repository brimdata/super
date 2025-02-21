package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type TypeValue struct {
	Offsets []uint32
	Bytes   []byte
	Nulls   *Bool
}

var _ Any = (*TypeValue)(nil)

func NewTypeValue(offs []uint32, bytes []byte, nulls *Bool) *TypeValue {
	return &TypeValue{Offsets: offs, Bytes: bytes, Nulls: nulls}
}

func NewTypeValueEmpty(length uint32, nulls *Bool) *TypeValue {
	return NewTypeValue(make([]uint32, 1, length+1), nil, nulls)
}

func (t *TypeValue) Append(v []byte) {
	t.Bytes = append(t.Bytes, v...)
	t.Offsets = append(t.Offsets, uint32(len(t.Bytes)))
}

func (t *TypeValue) Type() super.Type {
	return super.TypeType
}

func (t *TypeValue) Len() uint32 {
	return uint32(len(t.Offsets) - 1)
}

func (t *TypeValue) Value(slot uint32) []byte {
	return t.Bytes[t.Offsets[slot]:t.Offsets[slot+1]]
}

func (t *TypeValue) Serialize(b *zcode.Builder, slot uint32) {
	if t.Nulls.Value(slot) {
		b.Append(nil)
	} else {
		b.Append(t.Value(slot))
	}
}

func TypeValueValue(val Any, slot uint32) ([]byte, bool) {
	switch val := val.(type) {
	case *TypeValue:
		return val.Value(slot), val.Nulls.Value(slot)
	case *Const:
		if val.Nulls.Value(slot) {
			return nil, true
		}
		s, _ := val.AsBytes()
		return s, false
	case *Dict:
		if val.Nulls.Value(slot) {
			return nil, true
		}
		slot = uint32(val.Index[slot])
		return val.Any.(*TypeValue).Value(slot), false
	case *View:
		slot = val.Index[slot]
		return TypeValueValue(val.Any, slot)
	}
	panic(val)
}
