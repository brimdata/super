package csupio

import (
	"bytes"
	"errors"
	"io"
	"math"

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
	objects    []*csup.Object
	projection field.Projection
	readerAt   io.ReaderAt
	vals       []super.Value
}

func NewReader(zctx *super.Context, r io.Reader, fields []field.Path) (zio.Reader, error) {
	ra, ok := r.(io.ReaderAt)
	if !ok {
		return nil, errors.New("Super Columnar requires a seekable input")
	}
	objects, err := readObjects(ra)
	if err != nil {
		return nil, err
	}
	return &reader{
		zctx:       zctx,
		objects:    objects,
		projection: field.NewProjection(fields),
		readerAt:   ra,
	}, nil
}

func (r *reader) Read() (*super.Value, error) {
again:
	if len(r.vals) == 0 {
		if len(r.objects) == 0 {
			return nil, nil
		}
		o := r.objects[0]
		r.objects = r.objects[1:]
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

func readObjects(r io.ReaderAt) ([]*csup.Object, error) {
	var objects []*csup.Object
	var start int64
	for {
		// NewObject puts the right end to the passed in SectionReader and since
		// the end is unkown just pass MaxInt64.
		o, err := csup.NewObject(io.NewSectionReader(r, start, math.MaxInt64))
		if err != nil {
			if err == io.EOF && len(objects) > 0 {
				return objects, nil
			}
			return nil, err
		}
		objects = append(objects, o)
		start += int64(o.Size())
	}
}
