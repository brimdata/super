package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type Array struct {
	Typ     *super.TypeArray
	Offsets []uint32
	Values  Any
	Nulls   bitvec.Bits
}

var _ Any = (*Array)(nil)

func NewArray(typ *super.TypeArray, offsets []uint32, values Any, nulls bitvec.Bits) *Array {
	return &Array{Typ: typ, Offsets: offsets, Values: values, Nulls: nulls}
}

func (*Array) Kind() Kind {
	return KindArray
}

func (a *Array) Type() super.Type {
	return a.Typ
}

func (a *Array) Len() uint32 {
	return uint32(len(a.Offsets) - 1)
}

func (a *Array) Serialize(b *scode.Builder, slot uint32) {
	if a.Nulls.IsSet(slot) {
		b.Append(nil)
		return
	}
	off := a.Offsets[slot]
	b.BeginContainer()
	for end := a.Offsets[slot+1]; off < end; off++ {
		a.Values.Serialize(b, off)
	}
	b.EndContainer()
}

func ContainerOffset(val Any, slot uint32) (uint32, uint32, bool) {
	switch val := val.(type) {
	case *Array:
		return val.Offsets[slot], val.Offsets[slot+1], val.Nulls.IsSet(slot)
	case *Set:
		return val.Offsets[slot], val.Offsets[slot+1], val.Nulls.IsSet(slot)
	case *Map:
		return val.Offsets[slot], val.Offsets[slot+1], val.Nulls.IsSet(slot)
	case *View:
		slot = val.Index[slot]
		return ContainerOffset(val.Any, slot)
	}
	panic(val)
}

func Inner(val Any) Any {
	switch val := val.(type) {
	case *Array:
		return val.Values
	case *Set:
		return val.Values
	case *Dict:
		return Inner(val.Any)
	case *View:
		return Inner(val.Any)
	}
	panic(val)
}
