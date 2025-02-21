package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Set struct {
	Typ     *super.TypeSet
	Offsets []uint32
	Values  Any
	Nulls   *Bool
}

var _ Any = (*Set)(nil)

func NewSet(typ *super.TypeSet, offsets []uint32, values Any, nulls *Bool) *Set {
	return &Set{Typ: typ, Offsets: offsets, Values: values, Nulls: nulls}
}

func (s *Set) Type() super.Type {
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
	b.TransformContainer(super.NormalizeSet)
	b.EndContainer()
}
