package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"golang.org/x/sync/errgroup"
)

type FusionEncoder struct {
	typ      *super.TypeFusion
	values   Encoder
	subTypes Encoder
}

var _ Encoder = (*FusionEncoder)(nil)

func NewFusionEncoder(typ *super.TypeFusion) *FusionEncoder {
	return &FusionEncoder{
		typ:      typ,
		values:   NewEncoder(typ.Type),
		subTypes: NewPrimitiveEncoder(super.TypeType),
	}
}

func (f *FusionEncoder) Write(body scode.Bytes) {
	it := body.Iter()
	f.values.Write(it.Next())
	f.subTypes.Write(it.Next())
}

func (f *FusionEncoder) Emit(w io.Writer) error {
	if err := f.values.Emit(w); err != nil {
		return err
	}
	return f.subTypes.Emit(w)
}

func (f *FusionEncoder) Encode(group *errgroup.Group) {
	f.values.Encode(group)
	f.subTypes.Encode(group)
}

func (f *FusionEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, values := f.values.Metadata(cctx, off)
	off, subTypes := f.subTypes.Metadata(cctx, off)
	return off, cctx.enter(&Fusion{
		Values:   values,
		SubTypes: subTypes,
	})
}
