package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Record struct {
	loader RecordLoader
	Typ    *super.TypeRecord
	fields []Any
	length uint32
	nulls  bitvec.Bits
}

var _ Any = (*Record)(nil)

func NewRecord(typ *super.TypeRecord, fields []Any, length uint32, nulls bitvec.Bits) *Record {
	return &Record{Typ: typ, fields: fields, length: length, nulls: nulls}
}

func NewRecordWithLoader(typ *super.TypeRecord, loader RecordLoader, length uint32) *Record {
	return &Record{Typ: typ, loader: loader, length: length}
}

func (r *Record) Type() super.Type {
	return r.Typ
}

func (r *Record) Len() uint32 {
	return r.length
}

func (r *Record) Fields() []Any {
	if r.fields == nil {
		r.fields, r.nulls = r.loader.Load()
	}
	return r.fields
}

func (r *Record) Nulls() bitvec.Bits {
	if r.fields == nil {
		r.fields, r.nulls = r.loader.Load()
	}
	return r.nulls
}

func (r *Record) SetNulls(nulls bitvec.Bits) {
	r.nulls = nulls
}

func (r *Record) Serialize(b *zcode.Builder, slot uint32) {
	if r.Nulls().IsSet(slot) {
		b.Append(nil)
		return
	}
	b.BeginContainer()
	for _, f := range r.fields {
		f.Serialize(b, slot)
	}
	b.EndContainer()
}
