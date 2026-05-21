package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type TypeValueEncoder struct {
	cctx    *Context
	encoder *Uint32Encoder
}

func NewTypeValueEncoder(cctx *Context, vec *vector.TypeValue) Encoder {
	defs, ids := vec.TypeIDs()
	merger := super.NewTypeDefsMerger(cctx.TypeDefs(), defs)
	mapped := make([]uint32, 0, len(ids))
	for _, localID := range ids {
		mapped = append(mapped, merger.LookupID(localID))
	}
	return &TypeValueEncoder{cctx: cctx, encoder: NewUint32Encoder(mapped)}
}

func (t *TypeValueEncoder) Emit(w io.Writer) error {
	return t.encoder.Emit(w)
}

func (t *TypeValueEncoder) Encode(group *errgroup.Group) {
	t.encoder.Encode(group)
}

func (t *TypeValueEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, loc := t.encoder.Segment(off)
	return off, cctx.enter(&TypeValue{
		Location: loc,
	})
}
