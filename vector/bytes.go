package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Bytes struct {
	Offs  []uint32
	Bytes []byte
	Nulls *Bool
}

var _ Any = (*Bytes)(nil)

func NewBytes(offs []uint32, bytes []byte, nulls *Bool) *Bytes {
	return &Bytes{Offs: offs, Bytes: bytes, Nulls: nulls}
}

func NewBytesEmpty(length uint32, nulls *Bool) *Bytes {
	return NewBytes(make([]uint32, 1, length+1), nil, nulls)
}

func (b *Bytes) Append(v []byte) {
	b.Bytes = append(b.Bytes, v...)
	b.Offs = append(b.Offs, uint32(len(b.Bytes)))
}

func (b *Bytes) Type() super.Type {
	return super.TypeBytes
}

func (b *Bytes) Len() uint32 {
	return uint32(len(b.Offs) - 1)
}

func (b *Bytes) Serialize(builder *zcode.Builder, slot uint32) {
	builder.Append(b.Value(slot))
}

func (b *Bytes) Value(slot uint32) []byte {
	if b.Nulls.Value(slot) {
		return nil
	}
	return b.Bytes[b.Offs[slot]:b.Offs[slot+1]]
}

func BytesValue(val Any, slot uint32) ([]byte, bool) {
	switch val := val.(type) {
	case *Bytes:
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
		return val.Any.(*Bytes).Value(slot), false
	case *View:
		slot = val.Index[slot]
		return BytesValue(val.Any, slot)
	}
	panic(val)
}
