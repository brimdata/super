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
	offsets *offsetsEncoder
	count   uint32
}

var _ Encoder = (*ArrayEncoder)(nil)

func NewArrayEncoder(typ *super.TypeArray) *ArrayEncoder {
	return &ArrayEncoder{
		typ:     typ.Type,
		values:  NewEncoder(typ.Type),
		offsets: newOffsetsEncoder(),
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
	a.offsets.writeLen(len)
}

func (a *ArrayEncoder) Encode(group *errgroup.Group) {
	a.offsets.Encode(group)
	a.values.Encode(group)
}

func (a *ArrayEncoder) Emit(w io.Writer) error {
	if err := a.offsets.Emit(w); err != nil {
		return err
	}
	return a.values.Emit(w)
}

func (a *ArrayEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, lens := a.offsets.Segment(off)
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
			typ:     typ.Type,
			values:  NewEncoder(typ.Type),
			offsets: newOffsetsEncoder(),
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
