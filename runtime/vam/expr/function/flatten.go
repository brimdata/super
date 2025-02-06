package function

import (
	"github.com/brimdata/super"
	samfunc "github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type flatten struct {
	fn *samfunc.Flatten
}

func newFlatten(zctx *super.Context) *flatten {
	return &flatten{samfunc.NewFlatten(zctx)}
}

func (f *flatten) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	rtyp := super.TypeRecordOf(vec.Type())
	if rtyp == nil {
		return args[0]
	}
	builder := vector.NewDynamicBuilder()
	var b zcode.Builder
	for i := range vec.Len() {
		b.Truncate()
		vec.Serialize(&b, i)
		val := f.fn.Call(nil, []super.Value{super.NewValue(rtyp, b.Bytes().Body())})
		builder.Write(val)
	}
	return builder.Build()
}

type unflatten struct {
	fn *samfunc.Unflatten
}

func newUnflatten(zctx *super.Context) *unflatten {
	return &unflatten{samfunc.NewUnflatten(zctx)}
}

func (u *unflatten) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	typ := vec.Type()
	builder := vector.NewDynamicBuilder()
	var b zcode.Builder
	for i := range vec.Len() {
		b.Reset()
		vec.Serialize(&b, i)
		val := u.fn.Call(nil, []super.Value{super.NewValue(typ, b.Bytes().Body())})
		builder.Write(val)
	}
	return builder.Build()
}
