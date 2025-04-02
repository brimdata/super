package csup

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

func (f *FieldEncoder) Metadata(cctx *Context, off uint64) (uint64, Field) {
	off, id := f.values.Metadata(cctx, off)
	return off, Field{
		Name:   f.name,
		Values: id,
	}
}

func (f *FieldEncoder) Encode(group *errgroup.Group) {
	f.values.Encode(group)
}

func (f *FieldEncoder) Emit(w io.Writer) error {
	return f.values.Emit(w)
}
