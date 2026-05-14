package vbuild

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type bytesBuilder struct {
	typ   super.Type
	table vector.BytesTable
}

func newBytesBuilder(typ super.Type) Builder {
	return &bytesBuilder{typ: typ, table: vector.NewBytesTableEmpty(0)}
}

func (s *bytesBuilder) Write(vec vector.Any) {
	switch vec := vec.(type) {
	case *vector.View:
		table := bytesTableOf(vec.Any)
		for _, slot := range vec.Index {
			s.table.Append(table.Bytes(slot))
		}
	case *vector.Const:
		b := bytesTableOf(vec.Any).Bytes(0)
		for range vec.Len() {
			s.table.Append(b)
		}
	case *vector.Dict:
		table := bytesTableOf(vec.Any)
		for _, slot := range vec.Index {
			s.table.Append(table.Bytes(uint32(slot)))
		}
	case *vector.String, *vector.Bytes:
		table := bytesTableOf(vec)
		for i := range vec.Len() {
			s.table.Append(table.Bytes(i))
		}
	default:
		panic(vec)
	}
}

func bytesTableOf(vec vector.Any) vector.BytesTable {
	switch vec := vec.(type) {
	case *vector.String:
		return vec.Table()
	case *vector.Bytes:
		return vec.Table()
	case *vector.Empty:
		return vector.BytesTable{}
	default:
		panic(vec)
	}
}

func (s *bytesBuilder) Build(*super.Context) vector.Any {
	switch s.typ.ID() {
	case super.IDString:
		return vector.NewString(s.table)
	case super.IDBytes:
		return vector.NewBytes(s.table)
	default:
		panic(s.typ)
	}
}
