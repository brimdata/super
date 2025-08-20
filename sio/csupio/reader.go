package csupio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type reader struct {
	sctx       *super.Context
	stream     *stream
	projection field.Projection
	readerAt   io.ReaderAt
	vals       []super.Value
	cancel     context.CancelFunc
}

func NewReader(sctx *super.Context, r io.Reader, fields []field.Path) (sio.Reader, error) {
	ra, ok := r.(io.ReaderAt)
	if !ok {
		return nil, errors.New("Super Columnar requires a seekable input")
	}
	// CSUP autodetection requires that we return error on open if invalid format.
	if _, err := csup.ReadHeader(ra); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &reader{
		sctx:       sctx,
		stream:     &stream{ctx: ctx, r: ra},
		projection: field.NewProjection(fields),
		readerAt:   ra,
		cancel:     cancel,
	}, nil
}

func (r *reader) Read() (*super.Value, error) {
again:
	if len(r.vals) == 0 {
		hdr, off, err := r.stream.next()
		if hdr == nil || err != nil {
			return nil, err
		}
		o, err := csup.NewObjectFromHeader(io.NewSectionReader(r.readerAt, off, math.MaxInt64), *hdr)
		if err != nil {
			return nil, err
		}
		vec, err := vcache.NewObjectFromCSUP(o).Fetch(r.sctx, r.projection)
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
	r.cancel()
	if closer, ok := r.readerAt.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
