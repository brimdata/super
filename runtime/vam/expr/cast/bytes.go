package cast

import (
	"github.com/brimdata/super/vector"
)

func castToBytes(vec vector.Any, index []uint32) (vector.Any, []uint32, string, bool) {
	strVec, ok := vec.(*vector.String)
	if !ok {
		return nil, nil, "", false
	}
	out := vector.Any(vector.NewBytes(strVec.Table(), vector.NullsOf(strVec)))
	if index != nil {
		out = vector.Pick(out, index)
	}
	return out, nil, "", true
}
