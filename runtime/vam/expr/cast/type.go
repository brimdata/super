package cast

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
)

func castToType(sctx *super.Context, vec vector.Any, index []uint32) (vector.Any, []uint32, bool) {
	switch vec := vec.(type) {
	case *vector.TypeValue:
		return vec, nil, true
	case *vector.String:
		n := lengthOf(vec, index)
		out := vector.NewTypeValueEmpty(0, nil)
		var errs []uint32
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			if vec.Nulls.Value(idx) {
				if out.Nulls == nil {
					out.Nulls = vector.NewBoolEmpty(n, nil)
				}
				out.Nulls.Set(i)
				out.Append(nil)
				continue
			}
			s := vec.Value(idx)
			val, err := sup.ParseValue(sctx, s)
			if err != nil || val.Type().ID() != super.IDType {
				errs = append(errs, i)
				continue
			}
			out.Append(val.Bytes())
		}
		return out, errs, true
	default:
		return nil, nil, false
	}
}
