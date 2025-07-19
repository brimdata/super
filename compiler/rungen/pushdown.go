package rungen

import (
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zbuf"
)

type pushdown struct {
	dataFilter     dag.Expr
	metaFilter     dag.Expr
	builder        *Builder
	projection     field.Projection
	metaProjection field.Projection
	unordred       bool
}

var _ zbuf.Pushdown = (*pushdown)(nil)

func (p *pushdown) DataFilter() (expr.Evaluator, error) {
	if p.dataFilter == nil {
		return nil, nil
	}
	return p.builder.compileExpr(p.dataFilter)
}

func (p *pushdown) BSUPFilter() (*expr.BufferFilter, error) {
	if p.dataFilter == nil {
		return nil, nil
	}
	return CompileBufferFilter(p.builder.sctx(), p.dataFilter)
}

func (p *pushdown) MetaFilter() (expr.Evaluator, field.Projection, error) {
	if p.metaFilter == nil {
		return nil, nil, nil
	}
	e, err := p.builder.compileExpr(p.metaFilter)
	if err != nil {
		return nil, nil, err
	}
	return e, p.metaProjection, nil
}

func (p *pushdown) Projection() field.Projection {
	return p.projection
}

func (p *pushdown) Unordered() bool {
	return p.unordred
}

type deleter struct {
	zbuf.Pushdown
	builder    *Builder
	dataFilter dag.Expr
}

func (d *deleter) DataFilter() (expr.Evaluator, error) {
	// For a DeleteFilter Evaluator the pushdown gets wrapped in a unary !
	// expression so we get all values that don't match. We also add an error
	// and null check because we want to keep these values around.
	return d.builder.compileExpr(&dag.BinaryExpr{
		Kind: "BinaryExpr",
		Op:   "or",
		LHS: &dag.UnaryExpr{
			Kind:    "UnaryExpr",
			Op:      "!",
			Operand: d.dataFilter,
		},
		RHS: &dag.BinaryExpr{
			Kind: "BinaryExpr",
			Op:   "or",
			LHS:  &dag.IsNullExpr{Kind: "IsNullExpr", Expr: d.dataFilter},
			RHS:  &dag.Call{Kind: "Call", Name: "is_error", Args: []dag.Expr{d.dataFilter}},
		},
	})
}

func (d *deleter) BSUPFilter() (*expr.BufferFilter, error) {
	return nil, nil
}
