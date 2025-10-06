package cast

import (
	"github.com/brimdata/super/vector"
)

func castToBytes(vec vector.Any, index []uint32) (vector.Any, []uint32, string, bool) {
	var out vector.Any
	switch vec := vec.(type) {
	case *vector.Bytes:
		out = vec
	case *vector.String:
		out = vector.NewBytes(vec.Table(), vector.NullsOf(vec))
	default:
		return nil, nil, "", false
	}
	if index != nil {
		out = vector.Pick(out, index)
	}
	return out, nil, "", true
}
