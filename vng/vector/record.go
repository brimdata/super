package vector

import (
	"errors"
	"io"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zcode"
)

var ErrVectorMismatch = errors.New("zng record value doesn't match vector writer")

type RecordWriter []*FieldWriter

func NewRecordWriter(typ *zed.TypeRecord, spiller *Spiller) RecordWriter {
	var r RecordWriter
	for _, f := range typ.Fields {
		fw := &FieldWriter{
			name:   f.Name,
			values: NewWriter(f.Type, spiller),
		}
		r = append(r, fw)
	}
	return r
}

func (r RecordWriter) Write(body zcode.Bytes) error {
	it := body.Iter()
	for _, f := range r {
		if it.Done() {
			return ErrVectorMismatch
		}
		if err := f.write(it.Next()); err != nil {
			return err
		}
	}
	if !it.Done() {
		return ErrVectorMismatch
	}
	return nil
}

func (r RecordWriter) Flush(eof bool) error {
	// XXX we might want to arrange these flushes differently for locality
	for _, f := range r {
		if err := f.Flush(eof); err != nil {
			return err
		}
	}
	return nil
}

func (r RecordWriter) Metadata() Metadata {
	fields := make([]Field, 0, len(r))
	for _, field := range r {
		fields = append(fields, field.Metadata())
	}
	return &Record{fields}
}

type RecordReader struct {
	Names  []string
	Values []FieldReader
}

var _ Reader = (*RecordReader)(nil)

func NewRecordReader(record *Record, reader io.ReaderAt) (*RecordReader, error) {
	names := make([]string, 0, len(record.Fields))
	values := make([]FieldReader, 0, len(record.Fields))
	for _, field := range record.Fields {
		names = append(names, field.Name)
		fr, err := NewFieldReader(field, reader)
		if err != nil {
			return nil, err
		}
		values = append(values, *fr)
	}
	result := &RecordReader{
		Names:  names,
		Values: values,
	}
	return result, nil
}

func (r *RecordReader) Read(b *zcode.Builder) error {
	b.BeginContainer()
	for _, f := range r.Values {
		if err := f.Read(b); err != nil {
			return err
		}
	}
	b.EndContainer()
	return nil
}
