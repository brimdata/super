package function

import (
	"github.com/brimdata/super"
)

type Under struct {
	sctx     *super.Context
	downcast *downcast
}

func NewUnder(sctx *super.Context) *Under {
	return &Under{
		sctx:     sctx,
		downcast: &downcast{sctx, "under"},
	}
}

func (u *Under) Call(args []super.Value) super.Value {
	val := args[0]
	switch typ := args[0].Type().(type) {
	case *super.TypeNamed:
		return super.NewValue(typ.Type, val.Bytes())
	case *super.TypeError:
		return super.NewValue(typ.Type, val.Bytes())
	case *super.TypeFusion:
		val, errVal := u.downcast.defuse(typ, val.Bytes())
		if errVal != nil {
			return *errVal
		}
		return val
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
