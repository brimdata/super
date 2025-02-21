package vector

import (
	"github.com/brimdata/super/zcode"
)

type View struct {
	Any
	Index []uint32
}

var _ Any = (*View)(nil)

func NewView(val Any, index []uint32) Any {
	switch val := val.(type) {
	case *Bool:
		return NewBoolView(val, index)
	case *Const:
		return NewConst(val.val, uint32(len(index)), NewBoolView(val.Nulls, index))
	case *Dict:
		index2 := make([]byte, len(index))
		counts := make([]uint32, val.Any.Len())
		var nulls *Bool
		if val.Nulls != nil {
			nulls = NewBoolEmpty(uint32(len(index)), nil)
			for k, idx := range index {
				if val.Nulls.Value(idx) {
					nulls.Set(uint32(k))
				}
				v := val.Index[idx]
				index2[k] = v
				counts[v]++
			}
		} else {
			for k, idx := range index {
				v := val.Index[idx]
				index2[k] = v
				counts[v]++
			}
		}
		return NewDict(val.Any, index2, counts, nulls)
	case *Error:
		return NewError(val.Typ, NewView(val.Vals, index), NewBoolView(val.Nulls, index))
	case *Union:
		tags, values := viewForUnionOrDynamic(index, val.Tags, val.TagMap.Forward, val.Values)
		return NewUnion(val.Typ, tags, values, NewBoolView(val.Nulls, index))
	case *Dynamic:
		return NewDynamic(viewForUnionOrDynamic(index, val.Tags, val.TagMap.Forward, val.Values))
	case *View:
		index2 := make([]uint32, len(index))
		for k, idx := range index {
			index2[k] = uint32(val.Index[idx])
		}
		return &View{val.Any, index2}
	case *Named:
		// Wrapped View under Named so vector.Under still works.
		return &Named{val.Typ, NewView(val.Any, index)}
	}
	return &View{val, index}
}

func NewInverseView(vec Any, index []uint32) Any {
	var inverse []uint32
	for i := range vec.Len() {
		if len(index) > 0 && index[0] == i {
			index = index[1:]
			continue
		}
		inverse = append(inverse, i)
	}
	return NewView(vec, inverse)
}

func NewBoolView(vec *Bool, index []uint32) *Bool {
	if vec == nil {
		return nil
	}
	out := NewBoolEmpty(uint32(len(index)), nil)
	for k, slot := range index {
		if vec.Value(slot) {
			out.Set(uint32(k))
		}
	}
	if vec.Nulls != nil {
		out.Nulls = NewBoolView(vec.Nulls, index)
	}
	return out
}

func viewForUnionOrDynamic(index, tags, forward []uint32, values []Any) ([]uint32, []Any) {
	indexes := make([][]uint32, len(values))
	resultTags := make([]uint32, len(index))
	for k, index := range index {
		tag := tags[index]
		indexes[tag] = append(indexes[tag], forward[index])
		resultTags[k] = tag
	}
	results := make([]Any, len(values))
	for k := range results {
		results[k] = NewView(values[k], indexes[k])
	}
	return resultTags, results
}

func (v *View) Len() uint32 {
	return uint32(len(v.Index))
}

func (v *View) Serialize(b *zcode.Builder, slot uint32) {
	v.Any.Serialize(b, v.Index[slot])
}
