package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type String struct {
	table BytesTable
	Nulls bitvec.Bits
}

var _ Any = (*String)(nil)

func NewString(table BytesTable, nulls bitvec.Bits) *String {
	return &String{table: table, Nulls: nulls}
}

func NewStringEmpty(cap uint32, nulls bitvec.Bits) *String {
	return NewString(NewBytesTableEmpty(cap), nulls)
}

func (s *String) Append(v string) {
	s.table.Append([]byte(v))
}

func (s *String) Type() super.Type {
	return super.TypeString
}

func (s *String) Len() uint32 {
	return s.table.Len()
}

func (s *String) Value(slot uint32) string {
	return s.table.String(slot)
}

func (s *String) Table() BytesTable {
	return s.table
}

func (s *String) Serialize(b *scode.Builder, slot uint32) {
	if s.Nulls.IsSet(slot) {
		b.Append(nil)
	} else {
		b.Append(s.table.Bytes(slot))
	}
}

func StringValue(val Any, slot uint32) (string, bool) {
	switch val := val.(type) {
	case *String:
		if val.Nulls.IsSet(slot) {
			return "", true
		}
		return val.Value(slot), false
	case *Const:
		if val.Nulls.IsSet(slot) {
			return "", true
		}
		s, _ := val.AsString()
		return s, false
	case *Dict:
		if val.Nulls.IsSet(slot) {
			return "", true
		}
		return StringValue(val.Any, uint32(val.Index[slot]))
	case *View:
		return StringValue(val.Any, val.Index[slot])
	}
	panic(val)
}
