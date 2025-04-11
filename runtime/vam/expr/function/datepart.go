package function

import (
	"github.com/brimdata/super"
	samfunc "github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

//go:generate go run gendatepartfuncs.go

type DatePart struct {
	sctx *super.Context
}

func (d *DatePart) Call(args ...vector.Any) vector.Any {
	if args[0].Type().ID() != super.IDString {
		return vector.NewWrappedError(d.sctx, "date_part: string value required for part argument", args[0])
	}
	if args[1].Type().ID() != super.IDTime {
		return vector.NewWrappedError(d.sctx, "date_part: time value required for time argument", args[1])
	}
	partArg, timeArg := vector.Under(args[0]), vector.Under(args[1])
	c, ok := partArg.(*vector.Const)
	if !ok || !c.Nulls().IsZero() {
		return d.slow(partArg, timeArg)
	}
	fn := datePartFuncs[c.Value().Ptr().AsString()]
	if fn == nil {
		return vector.NewWrappedError(d.sctx, "date_part: unknown part name", args[0])
	}
	return fn(timeArg)
}

func (d *DatePart) slow(partArg, timeArg vector.Any) vector.Any {
	fn := samfunc.NewDatePart(d.sctx)
	var b zcode.Builder
	vb := vector.NewDynamicBuilder()
	for i := range partArg.Len() {
		b.Reset()
		partArg.Serialize(&b, i)
		partVal := super.NewValue(super.TypeString, b.Bytes().Body())
		b.Reset()
		timeArg.Serialize(&b, i)
		timeVal := super.NewValue(super.TypeTime, b.Bytes().Body())
		vb.Write(fn.Call(nil, []super.Value{partVal, timeVal}))
	}
	return vb.Build()
}
