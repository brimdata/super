package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Map struct {
	loader  Uint32Loader
	Typ     *super.TypeMap
	Keys    Any
	Values  Any
	Nulls   bitvec.Bits
	offsets []uint32
	length  uint32
}

var _ Any = (*Map)(nil)

func NewMap(typ *super.TypeMap, offsets []uint32, keys Any, values Any, nulls bitvec.Bits) *Map {
	return &Map{Typ: typ, offsets: offsets, Keys: keys, Values: values, Nulls: nulls, length: uint32(len(offsets) - 1)}
}

func (m *Map) Type() super.Type {
	return m.Typ
}

func (m *Map) Len() uint32 {
	return m.length
}

func (m *Map) Offsets() []uint32 {
	if m.offsets == nil {
		m.offsets, m.Nulls = m.loader.Load()
	}
	return m.offsets
}

func (m *Map) Serialize(b *zcode.Builder, slot uint32) {
	if m.Nulls.IsSet(slot) {
		b.Append(nil)
		return
	}
	offs := m.Offsets()
	off := offs[slot]
	b.BeginContainer()
	for end := offs[slot+1]; off < end; off++ {
		m.Keys.Serialize(b, off)
		m.Values.Serialize(b, off)
	}
	b.TransformContainer(super.NormalizeMap)
	b.EndContainer()
}
