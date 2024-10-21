package vector

import (
	"encoding/binary"

	zed "github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Map struct {
	Typ     *zed.TypeMap
	Offsets []uint32
	Keys    Any
	Values  Any
	Nulls   *Bool
}

var _ Any = (*Map)(nil)

func NewMap(typ *zed.TypeMap, offsets []uint32, keys Any, values Any, nulls *Bool) *Map {
	return &Map{Typ: typ, Offsets: offsets, Keys: keys, Values: values, Nulls: nulls}
}

func (m *Map) Type() zed.Type {
	return m.Typ
}

func (m *Map) Len() uint32 {
	return uint32(len(m.Offsets) - 1)
}

func (m *Map) Serialize(b *zcode.Builder, slot uint32) {
	if m.Nulls.Value(slot) {
		b.Append(nil)
		return
	}
	off := m.Offsets[slot]
	b.BeginContainer()
	for end := m.Offsets[slot+1]; off < end; off++ {
		m.Keys.Serialize(b, off)
		m.Values.Serialize(b, off)
	}
	b.TransformContainer(zed.NormalizeMap)
	b.EndContainer()
}

func (m *Map) AppendKey(b []byte, slot uint32) []byte {
	b = binary.NativeEndian.AppendUint64(b, uint64(m.Typ.ID()))
	if m.Nulls.Value(slot) {
		return append(b, 0)
	}
	off := m.Offsets[slot]
	for end := m.Offsets[slot+1]; off < end; off++ {
		b = append(b, 0)
		b = m.Keys.AppendKey(b, off)
		b = m.Values.AppendKey(b, off)
	}
	return b
}
