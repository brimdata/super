package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type TypeValue struct {
	table BytesTable
	Nulls bitvec.Bits
}

var _ Any = (*TypeValue)(nil)

func NewTypeValue(table BytesTable, nulls bitvec.Bits) *TypeValue {
	return &TypeValue{table: table, Nulls: nulls}
}

func NewTypeValueEmpty(cap uint32, nulls bitvec.Bits) *TypeValue {
	return NewTypeValue(NewBytesTableEmpty(cap), nulls)
}

func (t *TypeValue) Append(v []byte) {
	t.table.Append(v)
}

func (t *TypeValue) Type() super.Type {
	return super.TypeType
}

func (t *TypeValue) Len() uint32 {
	return t.table.Len()
}

func (t *TypeValue) Value(slot uint32) []byte {
	return t.table.Bytes(slot)
}

func (t *TypeValue) Table() BytesTable {
	return t.table
}

func (t *TypeValue) Serialize(b *zcode.Builder, slot uint32) {
	if t.Nulls.IsSet(slot) {
		b.Append(nil)
	} else {
		b.Append(t.Value(slot))
	}
}

func TypeValueValue(val Any, slot uint32) ([]byte, bool) {
	switch val := val.(type) {
	case *TypeValue:
		return val.Value(slot), val.Nulls.IsSet(slot)
	case *Const:
		if val.Nulls.IsSet(slot) {
			return nil, true
		}
		s, _ := val.AsBytes()
		return s, false
	case *Dict:
		if val.Nulls.IsSet(slot) {
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
