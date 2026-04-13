package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

type fusion struct {
	sctx     *super.Context
	downcast Caster
}

func newFusion(sctx *super.Context) *fusion {
	return &fusion{
		sctx:     sctx,
		downcast: NewDowncast(sctx, "fusion"),
	}
}

func (f *fusion) Call(args []super.Value) super.Value {
	superVal, subTypeVal := args[0], args[1]
	if _, ok := super.TypeUnder(subTypeVal.Type()).(*super.TypeOfType); !ok {
		return f.sctx.WrapError("fusion: super type argument not a type", subTypeVal)
	}
	subType, err := f.sctx.LookupByValue(subTypeVal.Bytes())
	if err != nil {
		panic(err)
	}
	if _, ok := f.downcast.Cast(superVal, subType); !ok {
		return f.sctx.WrapError("fusion: value not a supertype of subtype arg: "+sup.FormatType(subType), superVal)
	}
	typ := f.sctx.LookupTypeFusion(superVal.Type())
	var b scode.Builder
	super.BuildFusion(&b, superVal.Bytes(), subTypeVal.Bytes())
	return super.NewValue(typ, b.Bytes().Body())
}
