package cast

import (
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

func castToBytes(vec vector.Any, index []uint32) (vector.Any, []uint32, bool) {
	n := lengthOf(vec, index)
	nulls := vector.NullsOf(vec)
	if index != nil {
		nulls = nulls.Pick(index)
	}
	out := vector.NewBytesEmpty(n, nulls)
	var b zcode.Builder
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		b.Reset()
		vec.Serialize(&b, idx)
		out.Append(b.Bytes().Body())
	}
	return out, nil, true
}
