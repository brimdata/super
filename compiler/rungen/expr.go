package rungen

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/function"
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

func (b *Builder) compileExprWithEmpty(e dag.Expr) (expr.Evaluator, error) {
	if e == nil {
		return nil, nil
	}
	return b.compileExpr(e)
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

func (b *Builder) compileAssignment(node *dag.Assignment) (expr.Assignment, error) {
	lhs, err := b.compileLval(node.LHS)
	if err != nil {
		return expr.Assignment{}, err
	}
	rhs, err := b.compileExpr(node.RHS)
	if err != nil {
		return expr.Assignment{}, fmt.Errorf("rhs of assigment expression: %w", err)
	}
	return expr.Assignment{LHS: lhs, RHS: rhs}, err
}

func (b *Builder) compileCall(call *dag.CallExpr) (expr.Evaluator, error) {
	// First check if call is to a user defined function, otherwise check for
	// builtin function.
	var fn expr.Function
	if f, ok := b.funcs[call.Tag]; ok {
		var err error
		if fn, err = b.compileUDFCall(call.Tag, f); err != nil {
			return nil, err
		}
	} else {
		var err error
		fn, err = function.New(b.sctx(), call.Tag, len(call.Args))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", call.Tag, err)
		}
	}
	exprs, err := b.compileExprs(call.Args)
	if err != nil {
		return nil, fmt.Errorf("%s: bad argument: %w", call.Tag, err)
	}
	return expr.NewCall(fn, exprs), nil
}

func (b *Builder) compileUDFCall(tag string, f *dag.FuncDef) (expr.Function, error) {
	if fn, ok := b.compiledUDFs[tag]; ok { //XXX this doesn't work for stateful things
		return fn, nil
	}
	fn := expr.NewUDF(b.sctx(), b.funcs[tag].Name, f.Params)
	// We store compiled UDF calls here so as to avoid stack overflows on
	// recursive calls.
	b.compiledUDFs[tag] = fn
	var err error
	if fn.Body, err = b.compileExpr(f.Expr); err != nil {
		return nil, err
	}
	delete(b.compiledUDFs, tag)
	return fn, nil
}

func (b *Builder) compileExprs(in []dag.Expr) ([]expr.Evaluator, error) {
	var exprs []expr.Evaluator
	for _, e := range in {
		ev, err := b.compileExpr(e)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, ev)
	}
	return exprs, nil
}

func (b *Builder) compileRecordExpr(record *dag.RecordExpr) (expr.Evaluator, error) {
	var elems []expr.RecordElem
	for _, elem := range record.Elems {
		switch elem := elem.(type) {
		case *dag.Field:
			e, err := b.compileExpr(elem.Value)
			if err != nil {
				return nil, err
			}
			elems = append(elems, &expr.FieldElem{
				Name: elem.Name,
				Expr: e,
				Opt:  elem.Opt,
			})
		case *dag.None:
			noneType, err := b.lookupType(elem.Type)
			if err != nil {
				return nil, err
			}
			elems = append(elems, &expr.NoneElem{
				Name: elem.Name,
				Type: noneType,
			})
		case *dag.Spread:
			e, err := b.compileExpr(elem.Expr)
			if err != nil {
				return nil, err
			}
			elems = append(elems, &expr.SpreadElem{Expr: e})
		}
	}
	return expr.NewRecordExpr(b.sctx(), elems), nil
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
