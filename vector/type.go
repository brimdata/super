package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type TypeValue struct {
	table BytesTable
}

var _ Any = (*TypeValue)(nil)

func NewTypeValue(table BytesTable) *TypeValue {
	return &TypeValue{table}
}

func NewTypeValueEmpty(cap uint32) *TypeValue {
	return NewTypeValue(NewBytesTableEmpty(cap))
}

func (t *TypeValue) Append(v []byte) {
	t.table.Append(v)
}

func (*TypeValue) Kind() Kind {
	return KindType
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

func (t *TypeValue) Serialize(b *scode.Builder, slot uint32) {
	b.Append(t.Value(slot))
}

func TypeValueValue(val Any, slot uint32) []byte {
	switch val := val.(type) {
	case *TypeValue:
		return val.Value(slot)
	case *Const:
		s, _ := val.AsBytes()
		return s
	case *Dict:
		slot = uint32(val.Index[slot])
		return val.Any.(*TypeValue).Value(slot)
	case *View:
		slot = val.Index[slot]
		return TypeValueValue(val.Any, slot)
	}
	panic(val)
}
