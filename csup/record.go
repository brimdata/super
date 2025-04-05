package csup

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
