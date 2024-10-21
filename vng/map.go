package vng

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

type MapEncoder struct {
	keys    Encoder
	values  Encoder
	lengths *Int64Encoder
	count   uint32
}

func NewMapEncoder(typ *super.TypeMap) *MapEncoder {
	return &MapEncoder{
		keys:    NewEncoder(typ.KeyType),
		values:  NewEncoder(typ.ValType),
		lengths: NewInt64Encoder(),
	}
}

func (m *MapEncoder) Write(body zcode.Bytes) {
	m.count++
	var len int
	it := body.Iter()
	for !it.Done() {
		m.keys.Write(it.Next())
		m.values.Write(it.Next())
		len++
	}
	m.lengths.Write(int64(len))
}

func (m *MapEncoder) Emit(w io.Writer) error {
	if err := m.lengths.Emit(w); err != nil {
		return err
	}
	if err := m.keys.Emit(w); err != nil {
		return err
	}
	return m.values.Emit(w)
}

func (m *MapEncoder) Metadata(off uint64) (uint64, Metadata) {
	off, lens := m.lengths.Metadata(off)
	off, keys := m.keys.Metadata(off)
	off, vals := m.values.Metadata(off)
	return off, &Map{
		Lengths: lens.(*Primitive).Location,
		Keys:    keys,
		Values:  vals,
		Length:  m.count,
	}
}

func (m *MapEncoder) Encode(group *errgroup.Group) {
	m.lengths.Encode(group)
	m.keys.Encode(group)
	m.values.Encode(group)
}

type MapBuilder struct {
	Keys    Builder
	Values  Builder
	Lengths *Int64Decoder
}

var _ Builder = (*MapBuilder)(nil)

func NewMapBuilder(m *Map, r io.ReaderAt) (*MapBuilder, error) {
	keys, err := NewBuilder(m.Keys, r)
	if err != nil {
		return nil, err
	}
	values, err := NewBuilder(m.Values, r)
	if err != nil {
		return nil, err
	}
	return &MapBuilder{
		Keys:    keys,
		Values:  values,
		Lengths: NewInt64Decoder(m.Lengths, r),
	}, nil
}

func (m *MapBuilder) Build(b *zcode.Builder) error {
	len, err := m.Lengths.Next()
	if err != nil {
		return err
	}
	b.BeginContainer()
	for k := 0; k < int(len); k++ {
		if err := m.Keys.Build(b); err != nil {
			return err
		}
		if err := m.Values.Build(b); err != nil {
			return err
		}
	}
	b.EndContainer()
	return nil
}
