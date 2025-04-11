package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Array struct {
	loader  Uint32Loader
	Typ     *super.TypeArray
	offsets []uint32
	Values  Any
	nulls   bitvec.Bits
	length  uint32
}

var _ Any = (*Array)(nil)

func NewArray(typ *super.TypeArray, offsets []uint32, values Any, nulls bitvec.Bits) *Array {
	return &Array{Typ: typ, offsets: offsets, Values: values, nulls: nulls, length: uint32(len(offsets) - 1)}
}

func NewArrayWithLoader(typ *super.TypeArray, loader Uint32Loader, length uint32, values Any) *Array {
	return &Array{Typ: typ, loader: loader, Values: values, length: length}
}

func (a *Array) Type() super.Type {
	return a.Typ
}

func (a *Array) Len() uint32 {
	return a.length
}

func (a *Array) Offsets() []uint32 {
	if a.offsets == nil {
		a.offsets, a.nulls = a.loader.Load()
	}
	return a.offsets
}

func (a *Array) Nulls() bitvec.Bits {
	if a.offsets == nil {
		a.offsets, a.nulls = a.loader.Load()
	}
	return a.nulls
}

func (a *Array) SetNulls(nulls bitvec.Bits) {
	a.nulls = nulls
}

func (a *Array) Serialize(b *zcode.Builder, slot uint32) {
	if a.Nulls().IsSet(slot) {
		b.Append(nil)
		return
	}
	offs := a.Offsets()
	off := offs[slot]
	b.BeginContainer()
	for end := offs[slot+1]; off < end; off++ {
		a.Values.Serialize(b, off)
	}
	b.EndContainer()
}

func ContainerOffset(val Any, slot uint32) (uint32, uint32, bool) {
	switch val := val.(type) {
	case *Array:
		offs := val.Offsets()
		return offs[slot], offs[slot+1], val.Nulls().IsSet(slot)
	case *Set:
		offs := val.Offsets()
		return offs[slot], offs[slot+1], val.Nulls.IsSet(slot)
	case *Map:
		offs := val.Offsets()
		return offs[slot], offs[slot+1], val.Nulls.IsSet(slot)
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
