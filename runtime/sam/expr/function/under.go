package function

import (
	"github.com/brimdata/super"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#under
type Under struct {
	sctx *super.Context
}

func (u *Under) Call(args []super.Value) super.Value {
	val := args[0]
	switch typ := args[0].Type().(type) {
	case *super.TypeNamed:
		return super.NewValue(typ.Type, val.Bytes())
	case *super.TypeError:
		return super.NewValue(typ.Type, val.Bytes())
	case *super.TypeUnion:
		return super.NewValue(typ.Untag(val.Bytes()))
	case *super.TypeOfType:
		t, err := u.sctx.LookupByValue(val.Bytes())
		if err != nil {
			return u.sctx.NewError(err)
		}
		return u.sctx.LookupTypeValue(super.TypeUnder(t))
	default:
		return val
	}
}
