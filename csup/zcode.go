package csup

import (
	"io"
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

const MaxDictSize = 256

type ZcodeEncoder struct {
	typ   super.Type
	bytes zcode.Bytes
	cmp   expr.CompareFn
	min   *super.Value
	max   *super.Value
	count uint32

	// fields used after Encode is called
	bytesLen uint64
	format   uint8
	out      []byte
}

func NewZcodeEncoder(typ super.Type) *ZcodeEncoder {
	return &ZcodeEncoder{
		typ: typ,
		cmp: expr.NewValueCompareFn(order.Asc, false),
	}
}

func (p *ZcodeEncoder) Write(body zcode.Bytes) {
	p.update(body)
	p.bytes = zcode.Append(p.bytes, body)
}

func (p *ZcodeEncoder) update(body zcode.Bytes) {
	p.count++
	if body == nil {
		panic("PrimitiveWriter should not be called with null")
	}
	val := super.NewValue(p.typ, body)
	if p.min == nil || p.cmp(val, *p.min) < 0 {
		p.min = val.Copy().Ptr()
	}
	if p.max == nil || p.cmp(val, *p.max) > 0 {
		p.max = val.Copy().Ptr()
	}
}

func (p *ZcodeEncoder) Encode(group *errgroup.Group) {
	group.Go(func() error {
		fmt, out, err := compressBuffer(p.bytes)
		if err != nil {
			return err
		}
		p.format = fmt
		p.out = out
		p.bytesLen = uint64(len(p.bytes))
		p.bytes = nil // send to GC
		return nil
	})
}

func (p *ZcodeEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	loc := Segment{
		Offset:            off,
		Length:            uint64(len(p.out)),
		MemLength:         p.bytesLen,
		CompressionFormat: p.format,
	}
	off += uint64(len(p.out))
	return off, cctx.enter(&Primitive{
		Typ:      p.typ,
		Location: loc,
		Count:    p.count,
		Min:      p.min,
		Max:      p.max,
	})
}

func (p *ZcodeEncoder) Emit(w io.Writer) error {
	var err error
	if len(p.out) > 0 {
		_, err = w.Write(p.out)
	}
	return err
}

func (p *ZcodeEncoder) Dict() (PrimitiveEncoder, []byte, []uint32) {
	m := make(map[string]byte)
	var counts []uint32
	index := make([]byte, p.count)
	entries := NewZcodeEncoder(p.typ)
	var k uint32
	it := p.bytes.Iter()
	for !it.Done() {
		v := it.Next()
		tag, ok := m[string(v)]
		if !ok {
			tag = byte(len(counts))
			m[string(v)] = tag
			counts = append(counts, 0)
			entries.Write(v)
			if len(counts) > math.MaxUint8 {
				return nil, nil, nil
			}
		}
		index[k] = tag
		counts[tag]++
		k++
	}
	return entries, index, counts
}

func (p *ZcodeEncoder) ConstValue() super.Value {
	it := p.bytes.Iter()
	return super.NewValue(p.typ, it.Next())
}
