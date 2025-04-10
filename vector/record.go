package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Record struct {
	Typ    *super.TypeRecord
	Fields []Any
	len    uint32
	Nulls  bitvec.Bits
}

var _ Any = (*Record)(nil)

func NewRecord(typ *super.TypeRecord, fields []Any, length uint32, nulls bitvec.Bits) *Record {
	return &Record{Typ: typ, Fields: fields, len: length, Nulls: nulls}
}

func (r *Record) Type() super.Type {
	return r.Typ
}

func (r *Record) Len() uint32 {
	return r.len
}

func (r *Record) Serialize(b *zcode.Builder, slot uint32) {
	if r.Nulls.IsSet(slot) {
		b.Append(nil)
		return
	}
	b.BeginContainer()
	for _, f := range r.Fields {
		f.Serialize(b, slot)
	}
	b.EndContainer()
}
