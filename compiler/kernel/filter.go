package kernel

import (
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zbuf"
)

type Filter struct {
	pushdown dag.Expr
	builder  *Builder
}

var _ zbuf.Filter = (*Filter)(nil)

func (f *Filter) AsEvaluator() (expr.Evaluator, error) {
	if f == nil {
		return nil, nil
	}
	return f.builder.compileExpr(f.pushdown)
}

func (f *Filter) AsBufferFilter() (*expr.BufferFilter, error) {
	if f == nil {
		return nil, nil
	}
	return CompileBufferFilter(f.builder.zctx(), f.pushdown)
}

type DeleteFilter struct {
	*Filter
}

func (f *DeleteFilter) AsEvaluator() (expr.Evaluator, error) {
	if f == nil {
		return nil, nil
	}
	// For a DeleteFilter Evaluator the pushdown gets wrapped in a unary !
	// expression so we get all values that don't match. We also add an error
	// and null check because we want to keep these values around.
	return f.builder.compileExpr(&dag.BinaryExpr{
		Kind: "BinaryExpr",
		Op:   "or",
		LHS: &dag.UnaryExpr{
			Kind:    "UnaryExpr",
			Op:      "!",
			Operand: f.pushdown,
		},
		RHS: &dag.BinaryExpr{
			Kind: "BinaryExpr",
			Op:   "or",
			LHS:  &dag.IsNullExpr{Kind: "IsNullExpr", Expr: f.pushdown},
			RHS:  &dag.Call{Kind: "Call", Name: "is_error", Args: []dag.Expr{f.pushdown}},
		},
	})
}

func (f *DeleteFilter) AsBufferFilter() (*expr.BufferFilter, error) {
	return nil, nil
}
