package csup

import (
	"io"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"golang.org/x/sync/errgroup"
)

type FloatEncoder struct {
	typ      super.Type
	vals     []float64
	min, max float64
	out      []byte
	fmt      uint8
}

func NewFloatEncoder(typ super.Type, vals []float64) *FloatEncoder {
	return &FloatEncoder{typ: typ, vals: vals}
}

func (f *FloatEncoder) Encode(group *errgroup.Group) {
	group.Go(func() error {
		bytes := slices.Clone(byteconv.ReinterpretSlice[byte](f.vals))
		var err error
		f.fmt, f.out, err = compressBuffer(bytes)
		return err
	})
	if len(f.vals) > 0 {
		group.Go(func() error {
			f.min, f.max = minMax(f.vals)
			return nil
		})
	}
}

func (u *FloatEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	loc := Segment{
		Offset:            off,
		MemLength:         uint64(len(u.vals)) * 8,
		Length:            uint64(len(u.out)),
		CompressionFormat: u.fmt,
	}
	off += loc.Length
	return off, cctx.enter(&Float{
		Typ:      u.typ,
		Location: loc,
		Min:      u.min,
		Max:      u.max,
		Count:    uint32(len(u.vals)),
	})
}

func (u *FloatEncoder) Emit(w io.Writer) error {
	var err error
	if len(u.out) > 0 {
		_, err = w.Write(u.out)
	}
	return err
}
