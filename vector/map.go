package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type Map struct {
	Typ     *super.TypeMap
	Offsets []uint32
	Keys    Any
	Values  Any
	Nulls   bitvec.Bits
}

var _ Any = (*Map)(nil)

func NewMap(typ *super.TypeMap, offsets []uint32, keys Any, values Any, nulls bitvec.Bits) *Map {
	return &Map{Typ: typ, Offsets: offsets, Keys: keys, Values: values, Nulls: nulls}
}

func (*Map) Kind() Kind {
	return KindMap
}

func (m *Map) Type() super.Type {
	return m.Typ
}

func (m *Map) Len() uint32 {
	return uint32(len(m.Offsets) - 1)
}

func (m *Map) Serialize(b *scode.Builder, slot uint32) {
	if m.Nulls.IsSet(slot) {
		b.Append(nil)
		return
	}
	off := m.Offsets[slot]
	b.BeginContainer()
	for end := m.Offsets[slot+1]; off < end; off++ {
		m.Keys.Serialize(b, off)
		m.Values.Serialize(b, off)
	}
	b.TransformContainer(super.NormalizeMap)
	b.EndContainer()
}
