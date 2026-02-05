package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"golang.org/x/sync/errgroup"
)

type RecordEncoder struct {
	fields []*FieldEncoder
	count  uint32
	nopt   int
}

var _ Encoder = (*RecordEncoder)(nil)

func NewRecordEncoder(typ *super.TypeRecord) *RecordEncoder {
	fields := make([]*FieldEncoder, 0, len(typ.Fields))
	var nopt int
	for _, f := range typ.Fields {
		fields = append(fields, &FieldEncoder{
			name:   f.Name,
			values: NewEncoder(f.Type),
			opt:    f.Opt,
		})
		if f.Opt {
			nopt++
		}
	}
	return &RecordEncoder{fields: fields, nopt: nopt}
}

func (r *RecordEncoder) Write(body scode.Bytes) {
	index := r.count
	r.count++
	it := scode.NewRecordIter(body, r.nopt)
	for _, f := range r.fields {
		elem, none := it.Next(f.opt)
		if !none {
			f.write(elem, index)
		}
	}
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
