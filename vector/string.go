package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type String struct {
	table BytesTable
}

func NewString(table BytesTable) *String {
	return &String{table}
}

func NewStringEmpty(cap uint32) *String {
	return NewString(NewBytesTableEmpty(cap))
}

func (s *String) Append(v string) {
	s.table.Append([]byte(v))
}

func (*String) Kind() Kind {
	return KindString
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
	b.Append(s.table.Bytes(slot))
}

func StringValue(val Any, slot uint32) string {
	switch val := val.(type) {
	case *String:
		return val.Value(slot)
	case *Const:
		s, _ := val.AsString()
		return s
	case *Dict:
		return StringValue(val.Any, uint32(val.Index[slot]))
	case *View:
		return StringValue(val.Any, val.Index[slot])
	}
	panic(val)
}
