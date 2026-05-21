package csup

import (
	"io"

	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type FusionEncoder struct {
	values   Encoder
	subtypes Encoder
}

var _ Encoder = (*FusionEncoder)(nil)

func NewFusionEncoder(cctx *Context, vec *vector.Fusion) *FusionEncoder {
	return &FusionEncoder{
		values: NewEncoder(cctx, vec.Values),
		// Call NewTypeValueEncoder directly because we do not want subtypes
		// wrapped in a dict / const.
		subtypes: NewTypeValueEncoder(cctx, vec.Subtypes),
	}
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
