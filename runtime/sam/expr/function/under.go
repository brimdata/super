package function

import (
	"github.com/brimdata/super"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#under
type Under struct {
	zctx *super.Context
}

func (u *Under) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0]
	switch typ := args[0].Type().(type) {
	case *super.TypeNamed:
		return super.NewValue(typ.Type, val.Bytes())
	case *super.TypeError:
		return super.NewValue(typ.Type, val.Bytes())
	case *super.TypeUnion:
		return super.NewValue(typ.Untag(val.Bytes()))
	case *super.TypeOfType:
		t, err := u.zctx.LookupByValue(val.Bytes())
		if err != nil {
			return u.zctx.NewError(err)
		}
		return u.zctx.LookupTypeValue(super.TypeUnder(t))
	default:
		return val
	}
}
