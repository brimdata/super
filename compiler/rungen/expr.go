package rungen

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/runtime/expr"
	samexpr "github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

func (b *Builder) compileExpr(e dag.Expr) (samexpr.Evaluator, error) {
	vamEval, err := b.compileVamExpr(e)
	if err != nil {
		return nil, err
	}
	return &samEvaluator{sctx: b.sctx(), vamEval: vamEval}, nil
}

type samEvaluator struct {
	sctx    *super.Context
	vamEval expr.Evaluator
	builder scode.Builder
}

func (v *samEvaluator) Eval(val super.Value) super.Value {
	b := vector.NewValueBuilder(val.Type())
	b.Write(val.Bytes())
	vec := b.Build(v.sctx)
	vec = v.vamEval.Eval(vec)
	return vector.ValueAt(&v.builder, vec, 0).Copy()
}

func (b *Builder) compileLval(e dag.Expr) (*samexpr.Lval, error) {
	switch e := e.(type) {
	case *dag.DotExpr:
		lhs, err := b.compileLval(e.LHS)
		if err != nil {
			return nil, err
		}
		lhs.Elems = append(lhs.Elems, &samexpr.StaticLvalElem{Name: e.RHS})
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
		container.Elems = append(container.Elems, samexpr.NewExprLvalElem(b.sctx(), index))
		return container, nil
	case *dag.ThisExpr:
		var elems []samexpr.LvalElem
		for _, elem := range e.Chain {
			elems = append(elems, &samexpr.StaticLvalElem{Name: elem.ID})
		}
		return samexpr.NewLval(elems), nil
	}
	return nil, fmt.Errorf("internal error: invalid lval %#v", e)
}

func (b *Builder) compileSortExprs(sortExprs []dag.SortExpr) ([]samexpr.SortExpr, error) {
	var out []samexpr.SortExpr
	for _, se := range sortExprs {
		e, err := b.compileExpr(se.Key)
		if err != nil {
			return nil, err
		}
		out = append(out, samexpr.NewSortExpr(e, se.Order, se.Nulls))
	}
	return out, nil
}
