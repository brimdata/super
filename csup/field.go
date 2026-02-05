package csup

import (
	"io"

	"github.com/brimdata/super/scode"
	"golang.org/x/sync/errgroup"
)

type FieldEncoder struct {
	name   string
	values Encoder
	nones  *NonesEncoder
}

func (f *FieldEncoder) write(body scode.Bytes) {
	f.values.Write(body)
	if f.nones != nil {
		f.nones.touchValue()
	}
}

func (f *FieldEncoder) Metadata(cctx *Context, off uint64) (uint64, Field) {
	var nones Segment
	if f.nones != nil {
		off, nones = f.nones.runs.Segment(off)
	}
	var id ID
	off, id = f.values.Metadata(cctx, off)
	return off, Field{
		Name:   f.name,
		Values: id,
		Opt:    f.nones != nil,
		Nones:  nones,
	}
}

func (f *FieldEncoder) Encode(group *errgroup.Group) {
	f.values.Encode(group)
}

func (f *FieldEncoder) Emit(w io.Writer) error {
	return f.values.Emit(w)
}
