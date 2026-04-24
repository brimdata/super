package csup

import (
	"io"

	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type FieldEncoder struct {
	name   string
	values Encoder
	rle    []uint32
	nones  *Uint32Encoder
}

func (f *FieldEncoder) write(vec vector.Any) {
	if opt, ok := vec.(*vector.Optional); ok {
		// RLEs have the nice property that you can just concatenate them
		// to append two vectors.
		// XXX We currently compute the RLE from the Dynamic but Optional needs
		// to be updated to keep the RLEs around and materialize the Dynamic on demand.
		f.rle = append(f.rle, opt.RLE()...)
		vec = opt.Values[0]

	}
	f.values.Write(vec)
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
		Nones:  nones,
	}
}

func (f *FieldEncoder) Encode(group *errgroup.Group, count uint32) {
	//XXX we should just have a meta optional that encodes vector.Optional
	//if f.opt {
	//	f.nones = &Uint32Encoder{vals: f.rle}
	//	f.nones.Encode(group)
	//}
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
