package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type FusionEncoder struct {
	cctx        *Context
	typ         *super.TypeFusion
	values      Encoder
	subtypes    []uint32
	subtypesEnc *Uint32Encoder
}

var _ Encoder = (*FusionEncoder)(nil)

func NewFusionEncoder(cctx *Context, typ *super.TypeFusion) *FusionEncoder {
	return &FusionEncoder{
		cctx:   cctx,
		typ:    typ,
		values: NewEncoder(cctx, typ.Type),
	}
}

func (f *FusionEncoder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	fusion := vec.(*vector.Fusion)
	f.values.Write(fusion.Values)
	//XXX calling SubTypes is a slow path... we should have another
	// method on vector.TypeLoader that can just return the type IDs
	// as a slice and lookup the interned CSUP type table and copy
	// what is needed.
	for _, typ := range fusion.Subtypes() {
		f.subtypes = append(f.subtypes, f.cctx.lookupTypeID(fusion.Sctx, typ))
	}
}

func (f *FusionEncoder) Emit(w io.Writer) error {
	if err := f.values.Emit(w); err != nil {
		return err
	}
	return f.subtypesEnc.Emit(w)
}

func (f *FusionEncoder) Encode(group *errgroup.Group) {
	f.values.Encode(group)
	f.subtypesEnc = &Uint32Encoder{vals: f.subtypes}
	f.subtypesEnc.Encode(group)
}

func (f *FusionEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, values := f.values.Metadata(cctx, off)
	off, subtypes := f.subtypesEnc.Segment(off)
	return off, cctx.enter(&Fusion{
		Values:   values,
		Subtypes: subtypes,
	})
}
