package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Set struct {
	Typ     *super.TypeSet
	Offsets []uint32
	Values  Any
}

var _ Any = (*TypeValue)(nil)

func NewSet(typ *super.TypeSet, offsets []uint32, values Any) *Set {
	return &Set{typ, offsets, values}
}

func (*Set) Kind() Kind {
	return KindSet
}

func (s *Set) Type() super.Type {
	return s.Typ
}

func (s *Set) Len() uint32 {
	return uint32(len(s.Offsets) - 1)
}

func (s *Set) Serialize(b *scode.Builder, slot uint32) {
	off := s.Offsets[slot]
	b.BeginContainer()
	for end := s.Offsets[slot+1]; off < end; off++ {
		s.Values.Serialize(b, off)
	}
	b.TransformContainer(super.NormalizeSet)
	b.EndContainer()
}
