package csup

import (
	"io"

	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type FieldEncoder struct {
	name   string
	values Encoder
	opt    bool
	rle    vector.RLE
	nones  *Uint32Encoder
}

func (f *FieldEncoder) write(body scode.Bytes, slot uint32) {
	f.values.Write(body)
	if f.opt {
		f.rle.Touch(slot)
	}
}

func (f *FieldEncoder) Metadata(cctx *Context, off uint64) (uint64, Field) {
	var nones Segment
	if f.nones != nil {
		off, nones = f.nones.Segment(off)
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
		f.nones = &Uint32Encoder{vals: runs}
		f.nones.Encode(group)
	}
	f.values.Encode(group)
}

func (f *FieldEncoder) Emit(w io.Writer) error {
	if f.nones != nil {
		if err := f.nones.Emit(w); err != nil {
			return err
		}
	}
	return f.values.Emit(w)
}
