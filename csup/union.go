package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
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

func (u *UnionEncoder) Write(body zcode.Bytes) {
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

func (u *UnionEncoder) Metadata(off uint64) (uint64, Metadata) {
	off, tags := u.tags.Segment(off)
	values := make([]Metadata, 0, len(u.values))
	for _, val := range u.values {
		var meta Metadata
		off, meta = val.Metadata(off)
		values = append(values, meta)
	}
	return off, &Union{
		Tags:   tags,
		Values: values,
		Length: u.count,
	}
}
