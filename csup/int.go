package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/zcode"
	"github.com/ronanh/intcomp"
	"golang.org/x/sync/errgroup"
)

type IntEncoder struct {
	typ      super.Type
	vals     []int64
	min, max int64
	out      []byte
}

func NewIntEncoder(typ super.Type) *IntEncoder {
	return &IntEncoder{
		typ: typ,
	}
}

func (i *IntEncoder) Write(bytes zcode.Bytes) {
	v := super.DecodeInt(bytes)
	if len(i.vals) == 0 || v < i.min {
		i.min = v
	}
	if len(i.vals) == 0 || v > i.max {
		i.max = v
	}
	i.vals = append(i.vals, v)
}

func (i *IntEncoder) Encode(group *errgroup.Group) {
	group.Go(func() error {
		compressed := intcomp.CompressInt64(i.vals, nil)
		i.out = byteconv.ReinterpretSlice[byte](compressed)
		return nil
	})
}

func (i *IntEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	loc := Segment{
		Offset:            off,
		MemLength:         uint64(len(i.out)),
		Length:            uint64(len(i.vals)) * 8,
		CompressionFormat: CompressionFormatNone,
	}
	off += loc.MemLength
	return off, cctx.enter(&Int{
		Typ:      i.typ,
		Location: loc,
		Min:      i.min,
		Max:      i.max,
		Count:    uint32(len(i.vals)),
	})
}

func (i *IntEncoder) Emit(w io.Writer) error {
	var err error
	if len(i.out) > 0 {
		_, err = w.Write(i.out)
	}
	return err
}

func (i *IntEncoder) Dict() (PrimitiveEncoder, []byte, []uint32) {
	entries, index, counts := comparableDict(i.vals)
	if entries == nil {
		return nil, nil, nil
	}
	return &IntEncoder{
		typ:  i.typ,
		vals: entries,
		min:  i.min,
		max:  i.max,
	}, index, counts
}

func (i *IntEncoder) ConstValue() super.Value {
	return super.NewInt(i.typ, i.vals[0])
}

type UintEncoder struct {
	typ      super.Type
	vals     []uint64
	min, max uint64
	out      []byte
}

func NewUintEncoder(typ super.Type) *UintEncoder {
	return &UintEncoder{typ: typ}
}

func (u *UintEncoder) Write(bytes zcode.Bytes) {
	v := super.DecodeUint(bytes)
	if len(u.vals) == 0 || v < u.min {
		u.min = v
	}
	if len(u.vals) == 0 || v > u.max {
		u.max = v
	}
	u.vals = append(u.vals, v)
}

func (u *UintEncoder) Encode(group *errgroup.Group) {
	group.Go(func() error {
		compressed := intcomp.CompressUint64(u.vals, nil)
		u.out = byteconv.ReinterpretSlice[byte](compressed)
		return nil
	})
}

func (u *UintEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	loc := Segment{
		Offset:            off,
		MemLength:         uint64(len(u.out)),
		Length:            uint64(len(u.vals)) * 8,
		CompressionFormat: CompressionFormatNone,
	}
	off += loc.MemLength
	return off, cctx.enter(&Uint{
		Typ:      u.typ,
		Location: loc,
		Min:      u.min,
		Max:      u.max,
		Count:    uint32(len(u.vals)),
	})
}

func (u *UintEncoder) Emit(w io.Writer) error {
	var err error
	if len(u.out) > 0 {
		_, err = w.Write(u.out)
	}
	return err
}

func (u *UintEncoder) Dict() (PrimitiveEncoder, []byte, []uint32) {
	entries, index, counts := comparableDict(u.vals)
	if entries == nil {
		return nil, nil, nil
	}
	return &UintEncoder{
		typ:  u.typ,
		vals: entries,
		min:  u.min,
		max:  u.max,
	}, index, counts
}

func (u *UintEncoder) ConstValue() super.Value {
	return super.NewUint(u.typ, u.vals[0])
}

type Uint32Encoder struct {
	vals     []uint32
	out      []byte
	bytesLen uint64
}

func (u *Uint32Encoder) Write(v uint32) {
	u.vals = append(u.vals, v)
}

func (u *Uint32Encoder) Encode(group *errgroup.Group) {
	group.Go(func() error {
		u.bytesLen = uint64(len(u.vals) * 4)
		compressed := intcomp.CompressUint32(u.vals, nil)
		u.out = byteconv.ReinterpretSlice[byte](compressed)
		return nil
	})
}

func (u *Uint32Encoder) Emit(w io.Writer) error {
	var err error
	if len(u.out) > 0 {
		_, err = w.Write(u.out)
	}
	return err
}

func (u *Uint32Encoder) Segment(off uint64) (uint64, Segment) {
	len := uint64(len(u.out))
	return off + len, Segment{
		Offset:            off,
		MemLength:         len,
		Length:            u.bytesLen,
		CompressionFormat: CompressionFormatNone,
	}
}

type offsetsEncoder struct {
	Uint32Encoder
}

func newOffsetsEncoder() *offsetsEncoder {
	return &offsetsEncoder{
		Uint32Encoder{vals: []uint32{0}},
	}
}

func (o *offsetsEncoder) writeLen(size uint32) {
	o.vals = append(o.vals, o.vals[len(o.vals)-1]+size)
}

func ReadUint32s(loc Segment, r io.ReaderAt) ([]uint32, error) {
	buf := make([]byte, loc.MemLength)
	if err := loc.Read(r, buf); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	return intcomp.UncompressUint32(byteconv.ReinterpretSlice[uint32](buf), nil), nil
}
