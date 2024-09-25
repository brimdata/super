package vector

import (
	"encoding/binary"

	zed "github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Record struct {
	Typ    *zed.TypeRecord
	Fields []Any
	len    uint32
	Nulls  *Bool
}

var _ Any = (*Record)(nil)

func NewRecord(typ *zed.TypeRecord, fields []Any, length uint32, nulls *Bool) *Record {
	return &Record{Typ: typ, Fields: fields, len: length, Nulls: nulls}
}

func (r *Record) Type() zed.Type {
	return r.Typ
}

func (r *Record) Len() uint32 {
	return r.len
}

func (r *Record) Serialize(b *zcode.Builder, slot uint32) {
	if r.Nulls.Value(slot) {
		b.Append(nil)
		return
	}
	b.BeginContainer()
	for _, f := range r.Fields {
		f.Serialize(b, slot)
	}
	b.EndContainer()
}

func (r *Record) AppendKey(b []byte, slot uint32) []byte {
	b = binary.NativeEndian.AppendUint64(b, uint64(r.Typ.ID()))
	if r.Nulls.Value(slot) {
		return append(b, 0)
	}
	for _, f := range r.Fields {
		b = append(b, 0)
		b = f.AppendKey(b, slot)
	}
	return b
}
