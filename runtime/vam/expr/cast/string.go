package cast

import (
	"strconv"
	"time"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

func castToString(vec vector.Any, index []uint32) (vector.Any, []uint32, bool) {
	nulls := vector.NullsOf(vec)
	if index != nil {
		nulls = vector.NewBoolView(nulls, index)
	}
	n := lengthOf(vec, index)
	var bytes []byte
	offs := []uint32{0}
	switch vec := vec.(type) {
	case *vector.Int:
		switch vec.Type().ID() {
		case super.IDDuration:
			offs, bytes = durToString(vec, index, n)
		case super.IDTime:
			offs, bytes = timeToString(vec, index, n)
		default:
			for i := range n {
				idx := i
				if index != nil {
					idx = index[i]
				}
				bytes = strconv.AppendInt(bytes, vec.Values[idx], 10)
				offs = append(offs, uint32(len(bytes)))
			}
		}
	case *vector.Uint:
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			bytes = strconv.AppendUint(bytes, vec.Values[idx], 10)
			offs = append(offs, uint32(len(bytes)))
		}
	case *vector.Float:
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			bytes = strconv.AppendFloat(bytes, vec.Values[idx], 'g', -1, 64)
			offs = append(offs, uint32(len(bytes)))
		}
	case *vector.String:
		if index == nil {
			return vec, nil, true
		}
		for _, idx := range index {
			bytes = append(bytes, vec.Value(idx)...)
			offs = append(offs, uint32(len(bytes)))
		}
	case *vector.Bytes:
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			bytes = append(bytes, vec.Value(idx)...)
			offs = append(offs, uint32(len(bytes)))
		}
	case *vector.IP:
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			bytes = append(bytes, vec.Values[idx].String()...)
			offs = append(offs, uint32(len(bytes)))
		}
	case *vector.Net:
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			bytes = append(bytes, vec.Values[idx].String()...)
			offs = append(offs, uint32(len(bytes)))
		}
	case *vector.Enum:
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			if !nulls.Value(i) {
				val := vec.Uint.Values[idx]
				bytes = append(bytes, vec.Typ.Symbols[val]...)
			}
			offs = append(offs, uint32(len(bytes)))
		}
	default:
		var b zcode.Builder
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			b.Reset()
			vec.Serialize(&b, idx)
			v := super.NewValue(vec.Type(), b.Bytes().Body())
			bytes = append(bytes, sup.FormatValue(v)...)
			offs = append(offs, uint32(len(bytes)))
		}
	}
	return vector.NewString(offs, bytes, nulls), nil, true
}

func timeToString(vec *vector.Int, index []uint32, n uint32) ([]uint32, []byte) {
	var bytes []byte
	offs := []uint32{0}
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		s := nano.Ts(vec.Values[idx]).Time().Format(time.RFC3339Nano)
		bytes = append(bytes, s...)
		offs = append(offs, uint32(len(bytes)))
	}
	return offs, bytes
}

func durToString(vec *vector.Int, index []uint32, n uint32) ([]uint32, []byte) {
	var bytes []byte
	offs := []uint32{0}
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		bytes = append(bytes, nano.Duration(vec.Values[idx]).String()...)
		offs = append(offs, uint32(len(bytes)))
	}
	return offs, bytes
}
