package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

type ArrayEncoder struct {
	typ     super.Type
	values  Encoder
	lengths Uint32Encoder
	count   uint32
}

var _ Encoder = (*ArrayEncoder)(nil)

func NewArrayEncoder(typ *super.TypeArray) *ArrayEncoder {
	return &ArrayEncoder{
		typ:    typ.Type,
		values: NewEncoder(typ.Type),
	}
}

func (a *ArrayEncoder) Write(body zcode.Bytes) {
	a.count++
	it := body.Iter()
	var len uint32
	for !it.Done() {
		a.values.Write(it.Next())
		len++
	}
	a.lengths.Write(len)
}

func (a *ArrayEncoder) Encode(group *errgroup.Group) {
	a.lengths.Encode(group)
	a.values.Encode(group)
}

func (a *ArrayEncoder) Emit(w io.Writer) error {
	if err := a.lengths.Emit(w); err != nil {
		return err
	}
	return a.values.Emit(w)
}

func (a *ArrayEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, lens := a.lengths.Segment(off)
	off, vals := a.values.Metadata(cctx, off)
	return off, cctx.enter(&Array{
		Length:  a.count,
		Lengths: lens,
		Values:  vals,
	})
}

type SetEncoder struct {
	ArrayEncoder
}

func NewSetEncoder(typ *super.TypeSet) *SetEncoder {
	return &SetEncoder{
		ArrayEncoder{
			typ:    typ.Type,
			values: NewEncoder(typ.Type),
		},
	}
}

func (s *SetEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, id := s.ArrayEncoder.Metadata(cctx, off)
	array := cctx.Lookup(id).(*Array) // XXX this leaves a dummy node in the table
	return off, cctx.enter(&Set{
		Length:  array.Length,
		Lengths: array.Lengths,
		Values:  array.Values,
	})
}
