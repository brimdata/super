package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Array struct {
	Typ     *super.TypeArray
	Offsets []uint32
	Values  Any
}

var _ Any = (*Array)(nil)

func NewArray(typ *super.TypeArray, offsets []uint32, values Any) *Array {
	return &Array{typ, offsets, values}
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
	off := a.Offsets[slot]
	b.BeginContainer()
	for end := a.Offsets[slot+1]; off < end; off++ {
		a.Values.Serialize(b, off)
	}
	b.EndContainer()
}

func ContainerOffset(val Any, slot uint32) (uint32, uint32) {
	switch val := val.(type) {
	case *Array:
		return val.Offsets[slot], val.Offsets[slot+1]
	case *Set:
		return val.Offsets[slot], val.Offsets[slot+1]
	case *Map:
		return val.Offsets[slot], val.Offsets[slot+1]
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

func PushdownContainerView(val Any) Any {
	view, ok := val.(*View)
	if !ok {
		return val
	}
	switch val := view.Any.(type) {
	case *Array:
		inner, offsets := pickList(val.Values, view.Index, val.Offsets)
		return NewArray(val.Typ, offsets, inner)
	case *Set:
		inner, offsets := pickList(val.Values, view.Index, val.Offsets)
		return NewSet(val.Typ, offsets, inner)
	case *Map:
		keys, offsets := pickList(val.Keys, view.Index, val.Offsets)
		values, _ := pickList(val.Values, view.Index, val.Offsets)
		return NewMap(val.Typ, offsets, keys, values)
	default:
		panic(val)
	}
}

func pickList(inner Any, index, offsets []uint32) (Any, []uint32) {
	newOffsets := []uint32{0}
	var innerIndex []uint32
	for _, idx := range index {
		start, end := offsets[idx], offsets[idx+1]
		for ; start < end; start++ {
			innerIndex = append(innerIndex, start)
		}
		newOffsets = append(newOffsets, uint32(len(innerIndex)))
	}
	return Pick(inner, innerIndex), newOffsets
}
