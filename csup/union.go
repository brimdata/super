package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"golang.org/x/sync/errgroup"
)

type UnionEncoder struct {
	typ    *super.TypeUnion
	values []Encoder
	tags   Uint32Encoder
	count  uint32
}

var _ Encoder = (*UnionEncoder)(nil)

func NewUnionEncoder(typ *super.TypeUnion) *UnionEncoder {
	var values []Encoder
	for _, typ := range typ.Types {
		values = append(values, NewEncoder(typ))
	}
	return &UnionEncoder{
		typ:    typ,
		values: values,
	}
}

func (u *UnionEncoder) Write(body scode.Bytes) {
	u.count++
	typ, zv := u.typ.Untag(body)
	tag := u.typ.TagOf(typ)
	u.tags.Write(uint32(tag))
	u.values[tag].Write(zv)
}

func (u *UnionEncoder) Emit(w io.Writer) error {
	if err := u.tags.Emit(w); err != nil {
		return err
	}
	for _, value := range u.values {
		if err := value.Emit(w); err != nil {
			return err
		}
	}
	return nil
}

func (u *UnionEncoder) Encode(group *errgroup.Group) {
	u.tags.Encode(group)
	for _, value := range u.values {
		value.Encode(group)
	}
}

func (u *UnionEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, tags := u.tags.Segment(off)
	values := make([]ID, 0, len(u.values))
	for _, val := range u.values {
		var id ID
		off, id = val.Metadata(cctx, off)
		values = append(values, id)
	}
	return off, cctx.enter(&Union{
		Tags:   tags,
		Values: values,
		Length: u.count,
	})
}
