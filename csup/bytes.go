package csup

import (
	"bytes"
	"io"
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type BytesEncoder struct {
	typ      super.Type
	bytes    *BytesTableEncoder
	min, max []byte
}

func NewBytesEncoder(typ super.Type, table vector.BytesTable) *BytesEncoder {
	return &BytesEncoder{
		typ:   typ,
		bytes: &BytesTableEncoder{table: table},
	}
}

func (b *BytesEncoder) Encode(group *errgroup.Group) {
	b.bytes.Encode(group)
	group.Go(func() error {
		table := b.bytes.table
		if table.Len() == 0 {
			return nil
		}
		b.min, b.max = table.Bytes(0), table.Bytes(0)
		for i := range table.Len() {
			v := table.Bytes(i)
			if bytes.Compare(v, b.min) < 0 {
				b.min = v
			}
			if bytes.Compare(v, b.max) > 0 {
				b.max = v
			}
		}
		return nil
	})
}

func (b *BytesEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, bytesLoc, offsLoc := b.bytes.Segment(off)
	return off, cctx.enter(&Bytes{
		Typ:     b.typ,
		Bytes:   bytesLoc,
		Offsets: offsLoc,
		Min:     b.min,
		Max:     b.max,
		Count:   b.bytes.table.Len(),
	})
}

func (b *BytesEncoder) Emit(w io.Writer) error {
	return b.bytes.Emit(w)
}

func maybeStringBytesDict(typ super.Type, table vector.BytesTable) vector.Any {
	flat := func(table vector.BytesTable) vector.Any {
		if typ.ID() == super.IDString {
			return vector.NewString(table)
		}
		return vector.NewBytes(table)
	}
	var counts []uint32
	m := make(map[string]byte)
	index := make([]byte, table.Len())
	out := vector.NewBytesTableEmpty(0)
	for i := range table.Len() {
		tag, ok := m[string(table.Bytes(i))]
		if !ok {
			if len(counts) > math.MaxUint8 {
				return flat(table)
			}
			tag = byte(len(counts))
			b := table.Bytes(i)
			m[string(b)] = tag
			counts = append(counts, 0)
			out.Append(b)
		}
		index[i] = tag
		counts[tag]++
	}
	if !isValidDict(int(table.Len()), int(out.Len())) {
		return flat(table)
	}
	vec := flat(out)
	if vec.Len() == 1 {
		return vector.NewConst(vec, table.Len())
	}
	return vector.NewDict(vec, index, counts)
}

type BytesTableEncoder struct {
	table vector.BytesTable

	// These values are used for the Encode pass.
	bytesFmt uint8
	bytesOut []byte
	bytesLen uint64
	offsets  *Uint32Encoder
}

func NewBytesTableEncoder(table vector.BytesTable) *BytesTableEncoder {
	return &BytesTableEncoder{table: table}
}

func (b *BytesTableEncoder) Encode(group *errgroup.Group) {
	group.Go(func() error {
		bytes := b.table.RawBytes()
		fmt, out, err := compressBuffer(bytes)
		if err != nil {
			return err
		}
		b.bytesFmt = fmt
		b.bytesOut = out
		b.bytesLen = uint64(len(bytes))
		return nil
	})
	b.offsets = NewUint32Encoder(b.table.RawOffsets())
	b.offsets.Encode(group)
}

func (b *BytesTableEncoder) Segment(off uint64) (uint64, Segment, Segment) {
	bytesLoc := Segment{
		Offset:            off,
		Length:            uint64(len(b.bytesOut)),
		MemLength:         b.bytesLen,
		CompressionFormat: b.bytesFmt,
	}
	off, offsLoc := b.offsets.Segment(off + bytesLoc.Length)
	return off, bytesLoc, offsLoc
}

func (b *BytesTableEncoder) Emit(w io.Writer) error {
	if len(b.bytesOut) > 0 {
		if _, err := w.Write(b.bytesOut); err != nil {
			return err
		}
	}
	return b.offsets.Emit(w)
}
