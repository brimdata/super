package csup

import (
	"bytes"
	"io"
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type BytesEncoder struct {
	typ      super.Type
	min, max []byte
	bytes    scode.Bytes
	offsets  *offsetsEncoder

	// These values are used for the Encode pass.
	bytesFmt uint8
	bytesOut []byte
	bytesLen uint64
}

func NewBytesEncoder(typ super.Type) *BytesEncoder {
	return &BytesEncoder{
		typ:     typ,
		bytes:   scode.Bytes{},
		offsets: newOffsetsEncoder(),
	}
}

func (b *BytesEncoder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	switch vec := vec.(type) {
	case *vector.Bytes:
		b.writeTable(vec.Table())
	case *vector.String:
		b.writeTable(vec.Table())
	case *vector.TypeValue:
		b.writeTable(vec.Table())
	default:
		panic(vec)
	}
}

func (b *BytesEncoder) writeTable(table vector.BytesTable) {
	if len(b.bytes) == 0 {
		val := table.Bytes(0)
		b.min = append(b.min[:0], val...)
		b.max = append(b.max[:0], val...)
	}
	for slot := range table.Len() {
		val := table.Bytes(slot)
		if bytes.Compare(val, b.min) < 0 {
			b.min = append(b.min[:0], val...)
		}
		if bytes.Compare(val, b.max) > 0 {
			b.max = append(b.max[:0], val...)
		}
	}
	b.bytes = append(b.bytes, table.RawBytes()...)
	b.offsets.write(table.RawOffsets())
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

func (b *BytesEncoder) value(slot uint32) []byte {
	return b.bytes[b.offsets.vals[slot]:b.offsets.vals[slot+1]]
}

func (b *BytesEncoder) Dict() (PrimitiveEncoder, []byte, []uint32) {
	if len(b.bytes) == 0 {
		return nil, nil, nil
	}
	m := make(map[string]byte)
	var counts []uint32
	index := make([]byte, len(b.offsets.vals)-1)
	table := vector.NewBytesTableEmpty(256)
	for k := range uint32(len(index)) {
		tag, ok := m[string(b.value(k))]
		if !ok {
			tag = byte(len(counts))
			v := b.value(k)
			m[string(v)] = tag
			table.Append(v)
			counts = append(counts, 0)
			if len(counts) > math.MaxUint8 {
				return nil, nil, nil
			}
		}
		index[k] = tag
		counts[tag]++
	}
	encoder := NewBytesEncoder(b.typ)
	encoder.Write(vector.NewBytes(table))
	return encoder, index, counts
}

func (b *BytesEncoder) ConstValue() super.Value {
	return super.NewValue(b.typ, b.value(0))
}
