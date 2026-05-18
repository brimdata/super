package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type View struct {
	Any
	Index []uint32
}

var _ Any = (*View)(nil)

func NewView(vec Any, index []uint32) *View {
	return &View{vec, index}
}

func (v *View) Len() uint32 {
	return uint32(len(v.Index))
}

func (v *View) Serialize(b *scode.Builder, slot uint32) {
	v.Any.Serialize(b, v.Index[slot])
}

func PushView(vec Any) Any {
	view, ok := vec.(*View)
	if !ok {
		return vec
	}
	if view.Len() == 0 { //XXX
		return NewEmpty(view.Type())
	}
	switch vec := view.Any.(type) {
	case *Record:
		var fields []Any
		for _, field := range vec.Fields {
			fields = append(fields, Pick(field, view.Index))
		}
		return NewRecord(vec.Typ, fields, view.Len())
	case *Array:
		inner, offsets := pickList(vec.Values, view.Index, vec.Offsets)
		return NewArray(vec.Typ, offsets, inner)
	case *Set:
		inner, offsets := pickList(vec.Values, view.Index, vec.Offsets)
		return NewSet(vec.Typ, offsets, inner)
	case *Map:
		keys, offsets := pickList(vec.Keys, view.Index, vec.Offsets)
		values, _ := pickList(vec.Values, view.Index, vec.Offsets)
		return NewMap(vec.Typ, offsets, keys, values)
	case *Union:
		panic("TBD")
	case *Fusion:
		types := vec.Subtypes.Types()
		outTypes := make([]super.Type, len(view.Index))
		for i, slot := range view.Index {
			outTypes[i] = types[slot]
		}
		return NewFusion(vec.Sctx, vec.Typ, Pick(vec, view.Index), outTypes)
	default:
		return view
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
