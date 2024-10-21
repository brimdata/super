package vector

import (
	"encoding/binary"

	zed "github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Set struct {
	Typ     *zed.TypeSet
	Offsets []uint32
	Values  Any
	Nulls   *Bool
}

var _ Any = (*Set)(nil)

func NewSet(typ *zed.TypeSet, offsets []uint32, values Any, nulls *Bool) *Set {
	return &Set{Typ: typ, Offsets: offsets, Values: values, Nulls: nulls}
}

func (s *Set) Type() zed.Type {
	return s.Typ
}

func (s *Set) Len() uint32 {
	return uint32(len(s.Offsets) - 1)
}

func (s *Set) Serialize(b *zcode.Builder, slot uint32) {
	if s.Nulls.Value(slot) {
		b.Append(nil)
		return
	}
	off := s.Offsets[slot]
	b.BeginContainer()
	for end := s.Offsets[slot+1]; off < end; off++ {
		s.Values.Serialize(b, off)
	}
	b.TransformContainer(zed.NormalizeSet)
	b.EndContainer()
}

func (s *Set) AppendKey(b []byte, slot uint32) []byte {
	b = binary.NativeEndian.AppendUint64(b, uint64(s.Typ.ID()))
	if s.Nulls.Value(slot) {
		return append(b, 0)
	}
	off := s.Offsets[slot]
	for end := s.Offsets[slot+1]; off < end; off++ {
		b = append(b, 0)
		b = s.Values.AppendKey(b, off)
	}
	return b
}
