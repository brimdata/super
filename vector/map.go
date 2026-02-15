package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Map struct {
	Typ     *super.TypeMap
	Offsets []uint32
	Keys    Any
	Values  Any
}

func NewMap(typ *super.TypeMap, offsets []uint32, keys Any, values Any) *Map {
	return &Map{typ, offsets, keys, values}
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
	off := m.Offsets[slot]
	b.BeginContainer()
	for end := m.Offsets[slot+1]; off < end; off++ {
		m.Keys.Serialize(b, off)
		m.Values.Serialize(b, off)
	}
	b.TransformContainer(super.NormalizeMap)
	b.EndContainer()
}
