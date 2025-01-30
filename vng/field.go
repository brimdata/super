package vng

import (
	"io"

	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

type FieldEncoder struct {
	name   string
	values Encoder
}

func (f *FieldEncoder) write(body zcode.Bytes) {
	f.values.Write(body)
}

func (f *FieldEncoder) Metadata(off uint64) (uint64, Field) {
	off, meta := f.values.Metadata(off)
	return off, Field{
		Name:   f.name,
		Values: meta,
	}
}

func (f *FieldEncoder) Encode(group *errgroup.Group) {
	f.values.Encode(group)
}

func (f *FieldEncoder) Emit(w io.Writer) error {
	return f.values.Emit(w)
}
