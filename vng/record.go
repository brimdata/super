package vng

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

type RecordEncoder struct {
	fields []*FieldEncoder
	count  uint32
}

var _ Encoder = (*RecordEncoder)(nil)

func NewRecordEncoder(typ *super.TypeRecord) *RecordEncoder {
	fields := make([]*FieldEncoder, 0, len(typ.Fields))
	for _, f := range typ.Fields {
		fields = append(fields, &FieldEncoder{
			name:   f.Name,
			values: NewEncoder(f.Type),
		})
	}
	return &RecordEncoder{fields: fields}
}

func (r *RecordEncoder) Write(body zcode.Bytes) {
	r.count++
	it := body.Iter()
	for _, f := range r.fields {
		f.write(it.Next())
	}
}

func (r *RecordEncoder) Encode(group *errgroup.Group) {
	for _, f := range r.fields {
		f.Encode(group)
	}
}

func (r *RecordEncoder) Metadata(off uint64) (uint64, Metadata) {
	fields := make([]Field, 0, len(r.fields))
	for _, field := range r.fields {
		next, m := field.Metadata(off)
		fields = append(fields, m)
		off = next
	}
	return off, &Record{Length: r.count, Fields: fields}
}

func (r *RecordEncoder) Emit(w io.Writer) error {
	for _, f := range r.fields {
		if err := f.Emit(w); err != nil {
			return err
		}
	}
	return nil
}

type RecordBuilder struct {
	Names  []string
	Values []FieldBuilder
}

var _ Builder = (*RecordBuilder)(nil)

func NewRecordBuilder(record *Record, reader io.ReaderAt) (*RecordBuilder, error) {
	names := make([]string, 0, len(record.Fields))
	values := make([]FieldBuilder, 0, len(record.Fields))
	for _, field := range record.Fields {
		names = append(names, field.Name)
		fr, err := NewFieldBuilder(field, reader)
		if err != nil {
			return nil, err
		}
		values = append(values, *fr)
	}
	result := &RecordBuilder{
		Names:  names,
		Values: values,
	}
	return result, nil
}

func (r *RecordBuilder) Build(b *zcode.Builder) error {
	b.BeginContainer()
	for _, f := range r.Values {
		if err := f.Build(b); err != nil {
			return err
		}
	}
	b.EndContainer()
	return nil
}
