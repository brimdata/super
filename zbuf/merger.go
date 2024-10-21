package zbuf

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime/sam/expr"
)

func NewComparator(zctx *super.Context, sortKeys []order.SortKey) *expr.Comparator {
	exprs := make([]expr.SortEvaluator, len(sortKeys))
	for i, k := range sortKeys {
		exprs[i] = expr.NewSortEvaluator(expr.NewDottedExpr(zctx, k.Key), k.Order)
	}
	// valueAsBytes establishes a total order.
	exprs = append(exprs, expr.NewSortEvaluator(&valueAsBytes{}, order.Asc))
	nullsMax := sortKeys[0].Order == order.Asc
	return expr.NewComparator(nullsMax, exprs...).WithMissingAsNull()
}

func NewComparatorNullsMax(zctx *super.Context, sortKeys order.SortKeys) *expr.Comparator {
	exprs := make([]expr.SortEvaluator, len(sortKeys))
	for i, k := range sortKeys {
		exprs[i] = expr.NewSortEvaluator(expr.NewDottedExpr(zctx, k.Key), k.Order)
	}
	var o order.Which
	if !sortKeys.IsNil() {
		o = sortKeys.Primary().Order
	}
	// valueAsBytes establishes a total order.
	exprs = append(exprs, expr.NewSortEvaluator(&valueAsBytes{}, o))
	return expr.NewComparator(true, exprs...).WithMissingAsNull()
}

type valueAsBytes struct{}

func (v *valueAsBytes) Eval(ectx expr.Context, val super.Value) super.Value {
	return super.NewBytes(val.Bytes())
}
