package csup

import (
	"bytes"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

type BytesEncoder struct {
	typ      super.Type
	min, max []byte
	bytes    zcode.Bytes
	offsets  Uint32Encoder

	// These values are used for the Encode pass.
	bytesFmt uint8
	bytesOut []byte
	bytesLen uint64
}

func NewBytesEncoder(typ super.Type) *BytesEncoder {
	return &BytesEncoder{
		typ:     typ,
		bytes:   zcode.Bytes{},
		offsets: Uint32Encoder{vals: []uint32{0}},
	}
}

func (b *BytesEncoder) Write(vb zcode.Bytes) {
	if len(b.bytes) == 0 || bytes.Compare(vb, b.min) < 0 {
		b.min = append(b.min[:0], vb...)
	}
	if len(b.bytes) == 0 || bytes.Compare(vb, b.max) > 0 {
		b.max = append(b.max[:0], vb...)
	}
	b.bytes = append(b.bytes, vb...)
	b.offsets.Write(uint32(len(b.bytes)))
}

func (b *BytesEncoder) Encode(group *errgroup.Group) {
	group.Go(func() error {
		fmt, out, err := compressBuffer(b.bytes)
		if err != nil {
			return err
		}
		b.bytesFmt = fmt
		b.bytesOut = out
		b.bytesLen = uint64(len(b.bytes))
		b.bytes = nil // send to GC
		return nil
	})
	b.offsets.Encode(group)
}

func (b *BytesEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	bytesLoc := Segment{
		Offset:            off,
		Length:            uint64(len(b.bytesOut)),
		MemLength:         b.bytesLen,
		CompressionFormat: b.bytesFmt,
	}
	off, offsLoc := b.offsets.Segment(off + bytesLoc.Length)
	return off, cctx.enter(&Bytes{
		Typ:     b.typ,
		Bytes:   bytesLoc,
		Offsets: offsLoc,
		Min:     b.min,
		Max:     b.max,
		Count:   uint32(len(b.offsets.vals) - 1),
	})
}

func (b *BytesEncoder) Emit(w io.Writer) error {
	if len(b.bytesOut) > 0 {
		if _, err := w.Write(b.bytesOut); err != nil {
			return err
		}
	}
	return b.offsets.Emit(w)
}
