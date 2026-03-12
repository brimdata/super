package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
)

type Under struct {
	sctx     *super.Context
	downcast Caster
}

func NewUnder(sctx *super.Context) *Under {
	return &Under{
		sctx:     sctx,
		downcast: NewDowncast(sctx),
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
		it := val.Bytes().Iter()
		bytes := it.Next()
		subType, err := u.sctx.LookupByValue(it.Next())
		if err != nil {
			panic(err)
		}
		out, ok := u.downcast.Cast(super.NewValue(typ.Type, bytes), subType)
		if !ok {
			// The runtime should never allow creation of a super value that
			// doesn't follow the subtype invariant.
			panic(sup.FormatValue(val))
		}
		return out
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
