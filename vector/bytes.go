package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/scode"
)

type Bytes struct {
	table BytesTable
}

var _ Any = (*Bytes)(nil)

func NewBytes(table BytesTable) *Bytes {
	return &Bytes{table}
}

func NewBytesEmpty(cap uint32) *Bytes {
	return NewBytes(NewBytesTableEmpty(cap))
}

func (b *Bytes) Append(v []byte) {
	b.table.Append(v)
}

func (*Bytes) Kind() Kind {
	return KindBytes
}

func (b *Bytes) Type() super.Type {
	return super.TypeBytes
}

func (b *Bytes) Len() uint32 {
	return b.table.Len()
}

func (b *Bytes) Serialize(builder *scode.Builder, slot uint32) {
	builder.Append(b.Value(slot))
}

func (b *Bytes) Value(slot uint32) []byte {
	return b.table.Bytes(slot)
}

func (b *Bytes) Table() BytesTable {
	return b.table
}

func BytesValue(val Any, slot uint32) []byte {
	switch val := val.(type) {
	case *Bytes:
		return val.Value(slot)
	case *Const:
		s, _ := val.AsBytes()
		return s
	case *Dict:
		slot = uint32(val.Index[slot])
		return val.Any.(*Bytes).Value(slot)
	case *View:
		slot = val.Index[slot]
		return BytesValue(val.Any, slot)
	}
	panic(val)
}

type BytesTable struct {
	offsets []uint32
	bytes   []byte
}

func NewBytesTable(offsets []uint32, bytes []byte) BytesTable {
	return BytesTable{offsets, bytes}
}

func NewBytesTableEmpty(cap uint32) BytesTable {
	return BytesTable{make([]uint32, 1, cap+1), nil}
}

func (b BytesTable) Bytes(slot uint32) []byte {
	return b.bytes[b.offsets[slot]:b.offsets[slot+1]]
}

func (b BytesTable) String(slot uint32) string {
	return string(b.bytes[b.offsets[slot]:b.offsets[slot+1]])
}

func (b BytesTable) UnsafeString(slot uint32) string {
	return byteconv.UnsafeString(b.bytes[b.offsets[slot]:b.offsets[slot+1]])
}

func (b BytesTable) Slices() ([]uint32, []byte) {
	return b.offsets, b.bytes
}

func (b *BytesTable) Append(bytes []byte) {
	b.bytes = append(b.bytes, bytes...)
	b.offsets = append(b.offsets, uint32(len(b.bytes)))
}

func (b *BytesTable) Len() uint32 {
	if b.offsets == nil {
		return 0
	}
	return uint32(len(b.offsets) - 1)
}
