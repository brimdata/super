package csup

import (
	"io"

	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type FieldEncoder struct {
	name    string
	values  Encoder
	opt     bool
	rle     vector.RLE
	encoder *Uint32Encoder
}

func (f *FieldEncoder) write(body scode.Bytes, index uint32) {
	f.values.Write(body)
	if f.opt {
		f.rle.Touch(index)
	}
}

func (f *FieldEncoder) Metadata(cctx *Context, off uint64) (uint64, Field) {
	var nones Segment
	if f.encoder != nil {
		off, nones = f.encoder.Segment(off)
	}
	var id ID
	off, id = f.values.Metadata(cctx, off)
	return off, Field{
		Name:   f.name,
		Values: id,
		Opt:    f.opt,
		Nones:  nones,
	}
}

func (f *FieldEncoder) Encode(group *errgroup.Group, count uint32) {
	if f.opt {
		runs := f.rle.End(count)
		f.encoder = &Uint32Encoder{vals: runs}
		f.encoder.Encode(group)
	}
	f.values.Encode(group)
}

func (f *FieldEncoder) Emit(w io.Writer) error {
	if f.encoder != nil {
		if err := f.encoder.Emit(w); err != nil {
			return err
		}
	}
	return f.values.Emit(w)
}
