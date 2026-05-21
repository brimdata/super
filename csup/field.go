package csup

import (
	"io"

	"golang.org/x/sync/errgroup"
)

type FieldEncoder struct {
	name   string
	values Encoder
}

func (f *FieldEncoder) Metadata(cctx *Context, off uint64) (uint64, Field) {
	var id ID
	off, id = f.values.Metadata(cctx, off)
	return off, Field{
		Name:   f.name,
		Values: id,
	}
}

func (f *FieldEncoder) Encode(group *errgroup.Group, count uint32) {
	f.values.Encode(group)
}

func (f *FieldEncoder) Emit(w io.Writer) error {
	return f.values.Emit(w)
}
