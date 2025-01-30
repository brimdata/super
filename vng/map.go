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
	lengths Uint32Encoder
	count   uint32
}

func NewMapEncoder(typ *super.TypeMap) *MapEncoder {
	return &MapEncoder{
		keys:   NewEncoder(typ.KeyType),
		values: NewEncoder(typ.ValType),
	}
}

func (m *MapEncoder) Write(body zcode.Bytes) {
	m.count++
	var len uint32
	it := body.Iter()
	for !it.Done() {
		m.keys.Write(it.Next())
		m.values.Write(it.Next())
		len++
	}
	m.lengths.Write(len)
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
	off, lens := m.lengths.Segment(off)
	off, keys := m.keys.Metadata(off)
	off, vals := m.values.Metadata(off)
	return off, &Map{
		Lengths: lens,
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
