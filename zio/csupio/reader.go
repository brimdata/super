package csupio

import (
	"bytes"
	"errors"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zio"
)

type reader struct {
	zctx       *super.Context
	stream     *stream
	projection field.Projection
	readerAt   io.ReaderAt
	vals       []super.Value
}

func NewReader(zctx *super.Context, r io.Reader, fields []field.Path) (zio.Reader, error) {
	ra, ok := r.(io.ReaderAt)
	if !ok {
		return nil, errors.New("Super Columnar requires a seekable input")
	}
	// CSUP autodetection requires that we return error on open if invalid format.
	if _, err := csup.ReadHeader(ra); err != nil {
		return nil, err
	}
	return &reader{
		zctx:       zctx,
		stream:     &stream{r: ra},
		projection: field.NewProjection(fields),
		readerAt:   ra,
	}, nil
}

func (r *reader) Read() (*super.Value, error) {
again:
	if len(r.vals) == 0 {
		o, err := r.stream.next()
		if o == nil || err != nil {
			return nil, err
		}
		vec, err := vcache.NewObjectFromCSUP(o).Fetch(r.zctx, r.projection)
		if err != nil {
			return nil, err
		}
		r.materializeVector(vec)
		goto again
	}
	val := r.vals[0]
	r.vals = r.vals[1:]
	return &val, nil
}

func (r *reader) materializeVector(vec vector.Any) {
	r.vals = r.vals[:0]
	d, _ := vec.(*vector.Dynamic)
	var typ super.Type
	if d == nil {
		typ = vec.Type()
	}
	builder := zcode.NewBuilder()
	n := vec.Len()
	for slot := uint32(0); slot < n; slot++ {
		vec.Serialize(builder, slot)
		if d != nil {
			typ = d.TypeOf(slot)
		}
		val := super.NewValue(typ, bytes.Clone(builder.Bytes().Body()))
		r.vals = append(r.vals, val)
		builder.Truncate()
	}
}

func (r *reader) Close() error {
	if closer, ok := r.readerAt.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
