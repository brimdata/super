package vng

import (
	"errors"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

type UnionEncoder struct {
	typ    *super.TypeUnion
	values []Encoder
	tags   *Int64Encoder
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
		tags:   NewInt64Encoder(),
	}
}

func (u *UnionEncoder) Write(body zcode.Bytes) {
	u.count++
	typ, zv := u.typ.Untag(body)
	tag := u.typ.TagOf(typ)
	u.tags.Write(int64(tag))
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
	off, tags := u.tags.Metadata(off)
	values := make([]Metadata, 0, len(u.values))
	for _, val := range u.values {
		var meta Metadata
		off, meta = val.Metadata(off)
		values = append(values, meta)
	}
	return off, &Union{
		Tags:   tags.(*Primitive).Location,
		Values: values,
		Length: u.count,
	}
}

type UnionBuilder struct {
	builders []Builder
	tags     *Int64Decoder
}

var _ Builder = (*UnionBuilder)(nil)

func NewUnionBuilder(union *Union, r io.ReaderAt) (*UnionBuilder, error) {
	builders := make([]Builder, 0, len(union.Values))
	for _, val := range union.Values {
		b, err := NewBuilder(val, r)
		if err != nil {
			return nil, err
		}
		builders = append(builders, b)
	}
	return &UnionBuilder{
		builders: builders,
		tags:     NewInt64Decoder(union.Tags, r),
	}, nil
}

func (u *UnionBuilder) Build(b *zcode.Builder) error {
	tag, err := u.tags.Next()
	if err != nil {
		return err
	}
	if tag < 0 || int(tag) >= len(u.builders) {
		return errors.New("bad tag in VNG union builder")
	}
	b.BeginContainer()
	b.Append(super.EncodeInt(tag))
	if err := u.builders[tag].Build(b); err != nil {
		return err
	}
	b.EndContainer()
	return nil
}
