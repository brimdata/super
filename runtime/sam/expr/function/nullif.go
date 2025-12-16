package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime/sam/expr"
)

type NullIf struct {
	compareFn expr.CompareFn
}

func newNullIf() *NullIf {
	return &NullIf{expr.NewValueCompareFn(order.Asc, order.NullsLast)}
}

func (n *NullIf) Call(args []super.Value) super.Value {
	val0, val1 := args[0].Under(), args[1].Under()
	if val0.IsError() {
		return val0
	}
	if val1.IsError() {
		return val1
	}
	if n.compareFn(val0, val1) == 0 {
		return super.NewValue(val0.Type(), nil)
	}
	return val0
}
