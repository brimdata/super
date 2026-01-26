package vector

import "github.com/brimdata/super/vector/bitvec"

// Pick takes any vector vec and an index and returns a new vector consisting of the
// elements in the index.
func Pick(val Any, index []uint32) Any {
	switch val := val.(type) {
	case *Bool:
		return NewBool(val.Bits.Pick(index), val.Nulls.Pick(index))
	case *Const:
		return NewConst(val.val, uint32(len(index)), val.Nulls.Pick(index))
	case *Dict:
		index2 := make([]byte, len(index))
		counts := make([]uint32, val.Any.Len())
		var nulls bitvec.Bits
		if !val.Nulls.IsZero() {
			nulls = bitvec.NewFalse(uint32(len(index)))
			for k, idx := range index {
				if val.Nulls.IsSet(idx) {
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
		return NewError(val.Typ, Pick(val.Vals, index), val.Nulls.Pick(index))
	case *Union:
		tags, values := viewForUnionOrDynamic(index, val.Tags, val.ForwardTagMap(), val.Values)
		return NewUnion(val.Typ, tags, values, val.Nulls.Pick(index))
	case *Dynamic:
		return NewDynamic(viewForUnionOrDynamic(index, val.Tags, val.ForwardTagMap(), val.Values))
	case *View:
		index2 := make([]uint32, len(index))
		for k, idx := range index {
			index2[k] = uint32(val.Index[idx])
		}
		return NewView(val.Any, index2)
	case *Named:
		// Wrapped View under Named so vector.Under still works.
		return &Named{val.Typ, Pick(val.Any, index)}
	}
	return &View{val, index}
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
		results[k] = Pick(values[k], indexes[k])
	}
	return resultTags, results
}

// ReversePick is like Pick but it builds the vector from the elements
// that are not in the index maintaining the element order of vec.
func ReversePick(vec Any, index []uint32) Any {
	return Pick(vec, bitvec.ReverseIndex(index, vec.Len()))
}
