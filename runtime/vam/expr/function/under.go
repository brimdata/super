package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#under
type Under struct {
	sctx *super.Context
}

func (u *Under) Call(args ...vector.Any) vector.Any {
	vec := args[0]
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		vec, index = view.Any, view.Index
	}
	var out vector.Any
	switch vec := vec.(type) {
	case *vector.Named:
		out = vec.Any
	case *vector.Error:
		out = vec.Vals
	case *vector.Union:
		return vec.Dynamic
	case *vector.TypeValue:
		typs := vector.NewTypeValueEmpty(0, vec.Nulls)
		for i := range vec.Len() {
			if vec.Nulls.IsSet(i) {
				typs.Append(nil)
			}
			t, err := u.sctx.LookupByValue(vec.Value(i))
			if err != nil {
				panic(err)
			}
			v := u.sctx.LookupTypeValue(super.TypeUnder(t))
			typs.Append(v.Bytes())
		}
		out = typs
	default:
		return args[0]
	}
	if index != nil {
		return vector.Pick(out, index)
	}
	return out
}
