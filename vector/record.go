package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Record struct {
	Typ    *super.TypeRecord
	Fields []Any
	len    uint32
}

func NewRecord(typ *super.TypeRecord, fields []Any, length uint32) *Record {
	return &Record{typ, fields, length}
}

func (*Record) Kind() Kind {
	return KindRecord
}

func (r *Record) Type() super.Type {
	return r.Typ
}

func (r *Record) Len() uint32 {
	return r.len
}

func (r *Record) Serialize(b *scode.Builder, slot uint32) {
	b.BeginContainer()
	b.Append(nil) //XXX no optional fields... need to do this differently
	for _, f := range r.Fields {
		f.Serialize(b, slot)
	}
	b.EndContainer()
}
