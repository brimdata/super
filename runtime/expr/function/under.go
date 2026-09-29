package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Under struct{}

func (u *Under) Call(args ...vector.Any) vector.Any {
	vec := args[0]
	var index []uint32
	switch vec2 := vec.(type) {
	case *vector.Const:
		return vector.NewConst(u.Call(vec2.Any), vec.Len())
	case *vector.View:
		vec = vec2.Any
		index = vec2.Index
	}
	var out vector.Any
	switch vec := vec.(type) {
	case *vector.Named:
		out = vec.Any
	case *vector.Error:
		out = vec.Vals
	case *vector.Union:
		return vec.Dynamic()
	case *vector.TypeValue:
		typs := vector.NewTypeValueEmpty()
		for i := range vec.Len() {
			typs.Append(super.TypeUnder(vec.Value(i)))
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
