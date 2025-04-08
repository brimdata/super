package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime/sam/expr"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#compare
type Compare struct {
	nullsMax, nullsMin expr.CompareFn
	sctx               *super.Context
}

func NewCompare(sctx *super.Context) *Compare {
	return &Compare{
		nullsMax: expr.NewValueCompareFn(order.Asc, true),
		nullsMin: expr.NewValueCompareFn(order.Asc, false),
		sctx:     sctx,
	}
}

func (e *Compare) Call(_ super.Allocator, args []super.Value) super.Value {
	nullsMax := true
	if len(args) == 3 {
		if super.TypeUnder(args[2].Type()) != super.TypeBool {
			return e.sctx.WrapError("compare: nullsMax arg is not bool", args[2])
		}
		nullsMax = args[2].Bool()
	}
	if args[0].IsError() {
		return args[0]
	}
	if args[1].IsError() {
		return args[1]
	}
	cmp := e.nullsMax
	if !nullsMax {
		cmp = e.nullsMin
	}
	return super.NewInt64(int64(cmp(args[0], args[1])))
}
