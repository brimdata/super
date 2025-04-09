package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/zcode"
)

type Bytes struct {
	loader Loader
	table  BytesTable
	length uint32
	Nulls  *Bool
}

var _ Any = (*Bytes)(nil)

func NewBytes(table BytesTable, nulls *Bool) *Bytes {
	return &Bytes{table: table, length: table.Len(), Nulls: nulls}
}

func NewBytesEmpty(cap uint32, nulls *Bool) *Bytes {
	return NewBytes(NewBytesTableEmpty(cap), nulls)
}

func (b *Bytes) Append(v []byte) {
	b.table.append_(v)
	b.length = b.table.Len()
}

func (b *Bytes) Type() super.Type {
	return super.TypeBytes
}

func (b *Bytes) Len() uint32 {
	return b.length
}

func (b *Bytes) Serialize(builder *zcode.Builder, slot uint32) {
	builder.Append(b.Value(slot))
}

func (b *Bytes) Table() BytesTable {
	if b.table.offsets == nil {
		b.table = b.loader.Load().(BytesTable)
	}
	return b.table
}

func (b *Bytes) Value(slot uint32) []byte {
	if b.Nulls.Value(slot) {
		return nil
	}
	return b.Table().Bytes(slot)
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

func (b *BytesTable) append_(bytes []byte) {
	b.bytes = append(b.bytes, bytes...)
	b.offsets = append(b.offsets, uint32(len(b.bytes)))
}

func (b *BytesTable) Len() uint32 {
	if b.offsets == nil {
		return 0
	}
	return uint32(len(b.offsets) - 1)
}
