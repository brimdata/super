package function

import (
	"github.com/brimdata/super"
	samfunc "github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/vector"
)

type Under struct {
	sctx     *super.Context
	samunder *samfunc.Under
}

func newUnder(sctx *super.Context) *Under {
	return &Under{sctx, samfunc.NewUnder(sctx)}
}

func (u *Under) Call(args ...vector.Any) vector.Any {
	vec := args[0]
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		vec, index = view.Any, view.Index
	}
	var out vector.Any
	switch vec := vec.(type) {
	case *vector.Const:
		val := u.samunder.Call([]super.Value{vec.Value()})
		out = vector.NewConst(val, vec.Len())
	case *vector.Named:
		out = vec.Any
	case *vector.Error:
		out = vec.Vals
	case *vector.Union:
		return vec.Dynamic
	case *vector.TypeValue:
		typs := vector.NewTypeValueEmpty(0)
		for i := range vec.Len() {
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
