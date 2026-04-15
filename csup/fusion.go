package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type FusionEncoder struct {
	typ      *super.TypeFusion
	values   Encoder
	subtypes Encoder
}

var _ Encoder = (*FusionEncoder)(nil)

func NewFusionEncoder(cctx *Context, typ *super.TypeFusion) *FusionEncoder {
	return &FusionEncoder{
		typ:    typ,
		values: NewEncoder(cctx, typ.Type),
		// Call NewTypeValueEncoder directly because we do not want subtypes
		// wrapped in a dict / const.
		subtypes: NewTypeValueEncoder(cctx),
	}
}

func (f *FusionEncoder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	fusion := vec.(*vector.Fusion)
	f.values.Write(fusion.Values)
	f.subtypes.Write(fusion.Subtypes)
}

func (f *FusionEncoder) Emit(w io.Writer) error {
	if err := f.values.Emit(w); err != nil {
		return err
	}
	return f.subtypes.Emit(w)
}

func (f *FusionEncoder) Encode(group *errgroup.Group) {
	f.values.Encode(group)
	f.subtypes.Encode(group)
}

func (f *FusionEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, values := f.values.Metadata(cctx, off)
	off, subtypes := f.subtypes.Metadata(cctx, off)
	return off, cctx.enter(&Fusion{
		Values:   values,
		Subtypes: subtypes,
	})
}
