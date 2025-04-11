package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type String struct {
	loader BytesLoader
	table  BytesTable
	nulls  bitvec.Bits
	length uint32
}

var _ Any = (*String)(nil)

func NewString(table BytesTable, nulls bitvec.Bits) *String {
	return &String{table: table, nulls: nulls, length: table.Len()}
}

func NewStringEmpty(cap uint32, nulls bitvec.Bits) *String {
	return NewString(NewBytesTableEmpty(cap), nulls)
}

func NewStringWithLoader(table BytesTable, loader BytesLoader, length uint32) *String {
	return &String{table: table, loader: loader, length: length}
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
	return s.Table().String(slot)
}

func (s *String) Nulls() bitvec.Bits {
	if s.table.IsZero() {
		s.table, s.nulls = s.loader.Load()
	}
	return s.nulls
}

func (s *String) SetNulls(nulls bitvec.Bits) {
	s.nulls = nulls
}

func (s *String) Table() BytesTable {
	if s.table.IsZero() {
		s.table, s.nulls = s.loader.Load()
	}
	return s.table
}

func (s *String) Serialize(b *zcode.Builder, slot uint32) {
	if s.Nulls().IsSet(slot) {
		b.Append(nil)
	} else {
		b.Append(super.EncodeString(s.Value(slot)))
	}
}

func StringValue(val Any, slot uint32) (string, bool) {
	switch val := val.(type) {
	case *String:
		if val.Nulls().IsSet(slot) {
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
