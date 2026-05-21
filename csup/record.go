package csup

import (
	"io"

	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type RecordEncoder struct {
	fields []*FieldEncoder
	count  uint32
}

var _ Encoder = (*RecordEncoder)(nil)

func NewRecordEncoder(cctx *Context, vec *vector.Record) *RecordEncoder {
	fields := make([]*FieldEncoder, 0, len(vec.Fields))
	for i, f := range vec.Fields {
		fields = append(fields, &FieldEncoder{
			name:   vec.Typ.Fields[i].Name,
			values: NewEncoder(cctx, f),
		})
	}
	return &RecordEncoder{fields: fields, count: vec.Len()}
}

func (r *RecordEncoder) Encode(group *errgroup.Group) {
	for _, f := range r.fields {
		f.Encode(group, r.count)
	}
}

func (r *RecordEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	fields := make([]Field, 0, len(r.fields))
	for _, field := range r.fields {
		next, m := field.Metadata(cctx, off)
		fields = append(fields, m)
		off = next
	}
	return off, cctx.enter(&Record{Length: r.count, Fields: fields})
}

func (r *RecordEncoder) Emit(w io.Writer) error {
	for _, f := range r.fields {
		if err := f.Emit(w); err != nil {
			return err
		}
	}
	return nil
}
