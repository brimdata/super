package function

import (
	"github.com/brimdata/super"
	samfunc "github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

type flatten struct {
	sctx *super.Context
	fn   *samfunc.Flatten
}

func newFlatten(sctx *super.Context) *flatten {
	return &flatten{sctx, samfunc.NewFlatten(sctx)}
}

func (f *flatten) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	rtyp := super.TypeRecordOf(vec.Type())
	if rtyp == nil {
		return args[0]
	}
	builder := vector.NewDynamicValueBuilder()
	var b scode.Builder
	for i := range vec.Len() {
		b.Truncate()
		vec.Serialize(&b, i)
		val := f.fn.Call([]super.Value{super.NewValue(rtyp, b.Bytes().Body())})
		builder.Write(val)
	}
	return builder.Build(f.sctx)
}

type unflatten struct {
	sctx *super.Context
	fn   *samfunc.Unflatten
}

func newUnflatten(sctx *super.Context) *unflatten {
	return &unflatten{sctx, samfunc.NewUnflatten(sctx)}
}

func (u *unflatten) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	typ := vec.Type()
	builder := vector.NewDynamicValueBuilder()
	var b scode.Builder
	for i := range vec.Len() {
		b.Truncate()
		vec.Serialize(&b, i)
		val := u.fn.Call([]super.Value{super.NewValue(typ, b.Bytes().Body())})
		builder.Write(val)
	}
	return builder.Build(u.sctx)
}
