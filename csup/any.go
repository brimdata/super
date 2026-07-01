package csup

import (
	"io"

	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type AnyEncoder struct {
	values   Encoder
	subtypes Encoder
}

func NewAnyEncoder(cctx *Context, vec *vector.Fusion) *AnyEncoder {
	return &AnyEncoder{
		values:   NewPrimitiveEncoder(cctx, vec.Values, false),
		subtypes: NewTypeValueEncoder(cctx, vec.Subtypes),
	}
}

func (a *AnyEncoder) Emit(w io.Writer) error {
	if err := a.values.Emit(w); err != nil {
		return err
	}
	return a.subtypes.Emit(w)
}

func (a *AnyEncoder) Encode(group *errgroup.Group) {
	a.values.Encode(group)
	a.subtypes.Encode(group)
}

func (a *AnyEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, values := a.values.Metadata(cctx, off)
	off, subtypes := a.subtypes.Metadata(cctx, off)
	return off, cctx.enter(&Any{
		Values:   values,
		Subtypes: subtypes,
	})
}
