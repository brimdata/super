package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Set struct {
	loader  Uint32Loader
	Typ     *super.TypeSet
	Values  Any
	Nulls   bitvec.Bits
	offsets []uint32
	length  uint32
}

var _ Any = (*Set)(nil)

func NewSet(typ *super.TypeSet, offsets []uint32, values Any, nulls bitvec.Bits) *Set {
	return &Set{Typ: typ, offsets: offsets, Values: values, Nulls: nulls, length: uint32(len(offsets) - 1)}
}

func (s *Set) Type() super.Type {
	return s.Typ
}

func (s *Set) Len() uint32 {
	return s.length
}

func (s *Set) Offsets() []uint32 {
	if s.offsets == nil {
		s.offsets, s.Nulls = s.loader.Load()
	}
	return s.offsets
}

func (s *Set) Serialize(b *zcode.Builder, slot uint32) {
	if s.Nulls.IsSet(slot) {
		b.Append(nil)
		return
	}
	offs := s.Offsets()
	off := offs[slot]
	b.BeginContainer()
	for end := offs[slot+1]; off < end; off++ {
		s.Values.Serialize(b, off)
	}
	b.TransformContainer(super.NormalizeSet)
	b.EndContainer()
}
