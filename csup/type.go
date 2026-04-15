package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type TypeValueEncoder struct {
	cctx    *Context
	ids     []uint32
	encoder *Uint32Encoder
}

func NewTypeValueEncoder(cctx *Context) PrimitiveEncoder {
	return &TypeValueEncoder{cctx: cctx}
}

func (t *TypeValueEncoder) Write(vec vector.Any) {
	types := vec.(*vector.TypeValue)
	defs, ids := types.TypeIDs()
	merger := super.NewTypeDefsMerger(t.cctx.TypeDefs(), defs)
	mapped := make([]uint32, 0, len(ids))
	for _, localID := range ids {
		mapped = append(mapped, merger.LookupID(localID))
	}
	t.ids = append(t.ids, mapped...)
}

func (t *TypeValueEncoder) Emit(w io.Writer) error {
	return t.encoder.Emit(w)
}

func (t *TypeValueEncoder) Encode(group *errgroup.Group) {
	t.encoder = &Uint32Encoder{vals: t.ids}
	t.encoder.Encode(group)
}

func (t *TypeValueEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, loc := t.encoder.Segment(off)
	return off, cctx.enter(&TypeValue{
		Location: loc,
	})
}

func (t *TypeValueEncoder) Dict() (PrimitiveEncoder, []byte, []uint32) {
	entries, index, counts := comparableDict(t.ids)
	if entries == nil {
		return nil, nil, nil
	}
	return &TypeValueEncoder{
		ids: entries,
	}, index, counts
}

func (t *TypeValueEncoder) ConstValue() super.Value {
	typ := super.NewTypeDefsMapper(t.cctx.local, t.cctx.typedefs).LookupType(t.ids[0])
	return super.NewValue(super.TypeType, super.EncodeTypeValue(typ))
}
