package csup

import (
	"io"

	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
	"golang.org/x/sync/errgroup"
)

type BoolEncoder struct {
	bits bitvec.Bits

	// created on Encode
	out      []byte
	fmt      uint8
	bytesLen uint64
}

func NewBoolEncoder(vec *vector.Bool) Encoder {
	return &BoolEncoder{bits: vec.Bits}
}

func (b *BoolEncoder) Encode(group *errgroup.Group) {
	group.Go(func() error {
		bytes := byteconv.ReinterpretSlice[byte](b.bits.GetBits())
		f, out, err := compressBuffer(bytes)
		if err != nil {
			return err
		}
		b.fmt = f
		b.out = out
		b.bytesLen = uint64(len(bytes))
		return nil
	})
}

func (b *BoolEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	loc := Segment{
		Offset:            off,
		MemLength:         uint64(len(b.out)),
		Length:            b.bytesLen,
		CompressionFormat: b.fmt,
	}
	off += loc.Length
	return off, cctx.enter(&Bool{
		Location: loc,
		Count:    b.bits.Len(),
	})
}

func (b *BoolEncoder) Emit(w io.Writer) error {
	_, err := w.Write(b.out)
	return err
}
