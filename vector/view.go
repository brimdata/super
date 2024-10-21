package vector

import (
	"github.com/brimdata/super/zcode"
)

type View struct {
	Any
	Index []uint32
}

var _ Any = (*View)(nil)

func NewView(index []uint32, val Any) Any {
	switch val := val.(type) {
	case *Const:
		return NewConst(val.val, uint32(len(index)), nullsView(val.Nulls, index))
	case *Dict:
		index2 := make([]byte, len(index))
		var nulls *Bool
		if val.Nulls != nil {
			nulls = NewBoolEmpty(uint32(len(index)), nil)
			for k, idx := range index {
				if val.Nulls.Value(idx) {
					nulls.Set(uint32(k))
				}
				index2[k] = val.Index[idx]
			}
		} else {
			for k, idx := range index {
				index2[k] = val.Index[idx]
			}
		}
		return NewDict(val.Any, index2, nil, nulls)
	case *Error:
		return NewError(val.Typ, NewView(index, val.Vals), nullsView(val.Nulls, index))
	case *Union:
		tags, values := viewForUnionOrDynamic(index, val.Tags, val.TagMap.Forward, val.Values)
		return NewUnion(val.Typ, tags, values, nullsView(val.Nulls, index))
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
		return &Named{val.Typ, NewView(index, val.Any)}
	}
	return &View{val, index}
}

func nullsView(nulls *Bool, index []uint32) *Bool {
	if nulls == nil {
		return nil
	}
	var out *Bool
	for k, slot := range index {
		if nulls.Value(slot) {
			if out == nil {
				out = NewBoolEmpty(uint32(len(index)), nil)
			}
			out.Set(uint32(k))
		}
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
		results[k] = NewView(indexes[k], values[k])
	}
	return resultTags, results
}

func (v *View) Len() uint32 {
	return uint32(len(v.Index))
}

func (v *View) Serialize(b *zcode.Builder, slot uint32) {
	v.Any.Serialize(b, v.Index[slot])
}

func (v *View) AppendKey(b []byte, slot uint32) []byte {
	return v.Any.AppendKey(b, v.Index[slot])
}
