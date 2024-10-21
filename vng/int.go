package vng

import (
	"io"

	"github.com/brimdata/super"
)

type Int64Encoder struct {
	PrimitiveEncoder
}

func NewInt64Encoder() *Int64Encoder {
	return &Int64Encoder{*NewPrimitiveEncoder(super.TypeInt64, false)}
}

func (p *Int64Encoder) Write(v int64) {
	p.PrimitiveEncoder.Write(super.EncodeInt(v))
}

type Int64Decoder struct {
	PrimitiveBuilder
}

func NewInt64Decoder(loc Segment, r io.ReaderAt) *Int64Decoder {
	return &Int64Decoder{*NewPrimitiveBuilder(&Primitive{Typ: super.TypeInt64, Location: loc}, r)}
}

func (p *Int64Decoder) Next() (int64, error) {
	zv, err := p.ReadBytes()
	if err != nil {
		return 0, err
	}
	return super.DecodeInt(zv), err
}
