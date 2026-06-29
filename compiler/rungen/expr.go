package rungen

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/runtime/sam/expr"
	vamexpr "github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

func (b *Builder) compileExpr(e dag.Expr) (expr.Evaluator, error) {
	vamEval, err := b.compileVamExpr(e)
	if err != nil {
		return nil, err
	}
	return &samEvaluator{sctx: b.sctx(), vamEval: vamEval}, nil
}

type samEvaluator struct {
	sctx    *super.Context
	vamEval vamexpr.Evaluator
	builder scode.Builder
}

func (v *samEvaluator) Eval(val super.Value) super.Value {
	b := vector.NewValueBuilder(val.Type())
	b.Write(val.Bytes())
	vec := b.Build(v.sctx)
	vec = v.vamEval.Eval(vec)
	return vector.ValueAt(&v.builder, vec, 0).Copy()
}

func (b *Builder) compileLval(e dag.Expr) (*expr.Lval, error) {
	switch e := e.(type) {
	case *dag.DotExpr:
		lhs, err := b.compileLval(e.LHS)
		if err != nil {
			return nil, err
		}
		lhs.Elems = append(lhs.Elems, &expr.StaticLvalElem{Name: e.RHS})
		return lhs, nil
	case *dag.IndexExpr:
		container, err := b.compileLval(e.Expr)
		if err != nil {
			return nil, err
		}
		index, err := b.compileExpr(e.Index)
		if err != nil {
			return nil, err
		}
		container.Elems = append(container.Elems, expr.NewExprLvalElem(b.sctx(), index))
		return container, nil
	case *dag.ThisExpr:
		var elems []expr.LvalElem
		for _, elem := range e.Path {
			elems = append(elems, &expr.StaticLvalElem{Name: elem})
		}
		return expr.NewLval(elems), nil
	}
	return nil, fmt.Errorf("internal error: invalid lval %#v", e)
}

func (b *Builder) compileSortExprs(sortExprs []dag.SortExpr) ([]expr.SortExpr, error) {
	var out []expr.SortExpr
	for _, se := range sortExprs {
		e, err := b.compileExpr(se.Key)
		if err != nil {
			return nil, err
		}
		out = append(out, expr.NewSortExpr(e, se.Order, se.Nulls))
	}
	return out, nil
}
