package vng

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

const MaxDictSize = 256

type PrimitiveEncoder struct {
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

func NewPrimitiveEncoder(typ super.Type) *PrimitiveEncoder {
	return &PrimitiveEncoder{
		typ: typ,
		cmp: expr.NewValueCompareFn(order.Asc, false),
	}
}

func (p *PrimitiveEncoder) reset() {
	p.bytes, p.min, p.max, p.count = nil, nil, nil, 0
}

func (p *PrimitiveEncoder) Write(body zcode.Bytes) {
	p.update(body)
	p.bytes = zcode.Append(p.bytes, body)
}

func (p *PrimitiveEncoder) update(body zcode.Bytes) {
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

func (p *PrimitiveEncoder) Encode(group *errgroup.Group) {
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

func (p *PrimitiveEncoder) Metadata(off uint64) (uint64, Metadata) {
	loc := Segment{
		Offset:            off,
		Length:            uint64(len(p.out)),
		MemLength:         p.bytesLen,
		CompressionFormat: p.format,
	}
	off += uint64(len(p.out))
	return off, &Primitive{
		Typ:      p.typ,
		Location: loc,
		Count:    p.count,
		Min:      p.min,
		Max:      p.max,
	}
}

func (p *PrimitiveEncoder) Emit(w io.Writer) error {
	var err error
	if len(p.out) > 0 {
		_, err = w.Write(p.out)
	}
	return err
}
