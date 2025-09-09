package rungen

import (
	"errors"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/function"
	vamexpr "github.com/brimdata/super/runtime/vam/expr"
	vamfunction "github.com/brimdata/super/runtime/vam/expr/function"
	"github.com/brimdata/super/sup"
	"golang.org/x/text/unicode/norm"
)

func (b *Builder) compileVamExpr(e dag.Expr) (vamexpr.Evaluator, error) {
	if e == nil {
		return nil, errors.New("null expression not allowed")
	}
	switch e := e.(type) {
	case *dag.ArrayExpr:
		return b.compileVamArrayExpr(e)
	case *dag.Literal:
		val, err := sup.ParseValue(b.sctx(), e.Value)
		if err != nil {
			return nil, err
		}
		return vamexpr.NewLiteral(val), nil
	//case *dag.Var:
	//	return vamexpr.NewVar(e.Slot), nil
	case *dag.Search:
		return b.compileVamSearch(e)
	case *dag.This:
		return vamexpr.NewDottedExpr(b.sctx(), field.Path(e.Path)), nil
	case *dag.Dot:
		return b.compileVamDotExpr(e)
	case *dag.IndexExpr:
		return b.compileVamIndexExpr(e)
	case *dag.IsNullExpr:
		return b.compileVamIsNullExpr(e)
	case *dag.UnaryExpr:
		return b.compileVamUnary(*e)
	case *dag.BinaryExpr:
		return b.compileVamBinary(e)
	case *dag.Conditional:
		return b.compileVamConditional(*e)
	case *dag.Call:
		return b.compileVamCall(e)
	case *dag.Subquery:
		return b.compileVamSubquery(e)
	case *dag.RegexpMatch:
		return b.compileVamRegexpMatch(e)
	case *dag.RegexpSearch:
		return b.compileVamRegexpSearch(e)
	case *dag.RecordExpr:
		return b.compileVamRecordExpr(e)
	case *dag.SliceExpr:
		return b.compileVamSliceExpr(e)
	case *dag.SetExpr:
		return b.compileVamSetExpr(e)
	//case *dag.MapCall:
	//	return b.compileVamMapCall(e)
	//case *dag.MapExpr:
	//	return b.compileVamMapExpr(e)
	//case *dag.Agg:
	//	agg, err := b.compileAgg(e)
	//	if err != nil {
	//		return nil, err
	//	}
	//	return expr.NewAggregatorExpr(agg), nil
	//case *dag.OverExpr:
	//	return b.compileOverExpr(e)
	default:
		return nil, fmt.Errorf("vector expression type %T: not supported", e)
	}
}

func (b *Builder) compileVamExprWithEmpty(e dag.Expr) (vamexpr.Evaluator, error) {
	if e == nil {
		return nil, nil
	}
	return b.compileVamExpr(e)
}

func (b *Builder) compileVamBinary(e *dag.BinaryExpr) (vamexpr.Evaluator, error) {
	//XXX TBD
	//if e.Op == "in" {
	// Do a faster comparison if the LHS is a compile-time constant expression.
	//	if in, err := b.compileConstIn(e); in != nil && err == nil {
	//		return in, err
	//	}
	//}
	// XXX don't think we need this... callee can check for const
	//if e, err := b.compileVamConstCompare(e); e != nil && err == nil {
	//	return e, nil
	//}
	lhs, err := b.compileVamExpr(e.LHS)
	if err != nil {
		return nil, err
	}
	rhs, err := b.compileVamExpr(e.RHS)
	if err != nil {
		return nil, err
	}
	switch op := e.Op; op {
	case "and":
		return vamexpr.NewLogicalAnd(b.sctx(), lhs, rhs), nil
	case "or":
		return vamexpr.NewLogicalOr(b.sctx(), lhs, rhs), nil
	case "in":
		return vamexpr.NewIn(b.sctx(), lhs, rhs), nil
	case "==", "!=", "<", "<=", ">", ">=":
		return vamexpr.NewCompare(b.sctx(), lhs, rhs, op), nil
	case "+", "-", "*", "/", "%":
		return vamexpr.NewArith(b.sctx(), lhs, rhs, op), nil
	default:
		return nil, fmt.Errorf("invalid binary operator %s", op)
	}
}

func (b *Builder) compileVamConditional(node dag.Conditional) (vamexpr.Evaluator, error) {
	predicate, err := b.compileVamExpr(node.Cond)
	if err != nil {
		return nil, err
	}
	thenExpr, err := b.compileVamExpr(node.Then)
	if err != nil {
		return nil, err
	}
	elseExpr, err := b.compileVamExpr(node.Else)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewConditional(b.sctx(), predicate, thenExpr, elseExpr), nil
}

func (b *Builder) compileVamUnary(unary dag.UnaryExpr) (vamexpr.Evaluator, error) {
	e, err := b.compileVamExpr(unary.Operand)
	if err != nil {
		return nil, err
	}
	switch unary.Op {
	case "-":
		return vamexpr.NewUnaryMinus(b.sctx(), e), nil
	case "!":
		return vamexpr.NewLogicalNot(b.sctx(), e), nil
	default:
		return nil, fmt.Errorf("unknown unary operator %s", unary.Op)
	}
}

func (b *Builder) compileVamDotExpr(dot *dag.Dot) (vamexpr.Evaluator, error) {
	record, err := b.compileVamExpr(dot.LHS)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewDotExpr(b.sctx(), record, dot.RHS), nil
}

func (b *Builder) compileVamIndexExpr(idx *dag.IndexExpr) (vamexpr.Evaluator, error) {
	e, err := b.compileVamExpr(idx.Expr)
	if err != nil {
		return nil, err
	}
	index, err := b.compileVamExpr(idx.Index)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewIndexExpr(b.sctx(), e, index), nil
}

func (b *Builder) compileVamIsNullExpr(idx *dag.IsNullExpr) (vamexpr.Evaluator, error) {
	e, err := b.compileVamExpr(idx.Expr)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewIsNull(e), nil
}

func (b *Builder) compileVamExprs(in []dag.Expr) ([]vamexpr.Evaluator, error) {
	var exprs []vamexpr.Evaluator
	for _, e := range in {
		ev, err := b.compileVamExpr(e)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, ev)
	}
	return exprs, nil
}

func (b *Builder) compileVamCall(call *dag.Call) (vamexpr.Evaluator, error) {
	name := call.Name()
	if name == "cast" {
		return b.compileVamCast(call.Args)
	}
	if tf := expr.NewShaperTransform(name); tf != 0 {
		return b.compileVamShaper(call.Args, tf)
	}
	var fn vamexpr.Function
	if u, ok := b.udfs[name]; ok {
		var err error
		if fn, err = b.compileVamUDFCall(name, u); err != nil {
			return nil, err
		}
	} else {
		var err error
		fn, err = vamfunction.New(b.sctx(), name, len(call.Args))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	exprs, err := b.compileVamExprs(call.Args)
	if err != nil {
		return nil, err
	}
	// Any call that expects zero arguments must take one argument
	// consisting of a vector that can represent the length of the argument
	// vector so we just pass in "this".
	if len(exprs) == 0 {
		exprs = []vamexpr.Evaluator{vamexpr.NewDottedExpr(b.sctx(), nil)}
	}
	return vamexpr.NewCall(fn, exprs), nil
}

func (b *Builder) compileVamUDFCall(name string, lambda *dag.Lambda) (vamexpr.Function, error) {
	if fn, ok := b.compiledVamUDFs[name]; ok {
		return fn, nil
	}
	fn := vamexpr.NewUDF(b.sctx(), name, lambda.Formals)
	// We store compiled UDF calls here so as to avoid stack overflows on
	// recursive calls.
	b.compiledVamUDFs[name] = fn
	var err error
	if fn.Body, err = b.compileVamExpr(lambda.Expr); err != nil {
		return nil, err
	}
	delete(b.compiledUDFs, name)
	return fn, nil
}

func (b *Builder) compileVamCast(args []dag.Expr) (vamexpr.Evaluator, error) {
	if err := function.CheckArgCount(len(args), 2, 2); err != nil {
		return nil, err
	}
	exprs, err := b.compileVamExprs(args)
	if err != nil {
		return nil, err
	}
	if literal, ok := exprs[1].(*vamexpr.Literal); ok {
		if cast, err := vamexpr.NewLiteralCast(b.sctx(), exprs[0], literal); err == nil {
			return cast, nil
		}
	}
	return b.compileVamShaper(args, expr.Cast)
}

func (b *Builder) compileVamShaper(args []dag.Expr, tf expr.ShaperTransform) (vamexpr.Evaluator, error) {
	shaper, err := b.compileShaper(args, tf)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewSamExpr(shaper), nil
}

func (b *Builder) compileVamRecordExpr(e *dag.RecordExpr) (vamexpr.Evaluator, error) {
	var elems []vamexpr.RecordElem
	for _, elem := range e.Elems {
		var name string
		var dagExpr dag.Expr
		switch elem := elem.(type) {
		case *dag.Field:
			name = elem.Name
			dagExpr = elem.Value
		case *dag.Spread:
			name = ""
			dagExpr = elem.Expr
		default:
			panic(elem)
		}
		expr, err := b.compileVamExpr(dagExpr)
		if err != nil {
			return nil, err
		}
		elems = append(elems, vamexpr.RecordElem{
			Name: name,
			Expr: expr,
		})
	}
	return vamexpr.NewRecordExpr(b.sctx(), elems), nil
}

func (b *Builder) compileVamSubquery(query *dag.Subquery) (vamexpr.Evaluator, error) {
	e, err := b.compileSubquery(query)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewSamExpr(e), nil
}

func (b *Builder) compileVamRegexpMatch(match *dag.RegexpMatch) (vamexpr.Evaluator, error) {
	e, err := b.compileVamExpr(match.Expr)
	if err != nil {
		return nil, err
	}
	re, err := expr.CompileRegexp(match.Pattern)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewRegexpMatch(re, e), nil
}

func (b *Builder) compileVamRegexpSearch(search *dag.RegexpSearch) (vamexpr.Evaluator, error) {
	e, err := b.compileVamExpr(search.Expr)
	if err != nil {
		return nil, err
	}
	re, err := expr.CompileRegexp(search.Pattern)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewSearchRegexp(re, e), nil
}

func (b *Builder) compileVamSearch(search *dag.Search) (vamexpr.Evaluator, error) {
	val, err := sup.ParseValue(b.sctx(), search.Value)
	if err != nil {
		return nil, err
	}
	e, err := b.compileVamExpr(search.Expr)
	if err != nil {
		return nil, err
	}
	if super.TypeUnder(val.Type()) == super.TypeString {
		// Do a grep-style substring search instead of an
		// exact match on each value.
		term := norm.NFC.Bytes(val.Bytes())
		return vamexpr.NewSearchString(string(term), e), nil
	}
	return vamexpr.NewSearch(search.Text, val, e), nil
}

func (b *Builder) compileVamSliceExpr(slice *dag.SliceExpr) (vamexpr.Evaluator, error) {
	e, err := b.compileVamExpr(slice.Expr)
	if err != nil {
		return nil, err
	}
	from, err := b.compileVamExprWithEmpty(slice.From)
	if err != nil {
		return nil, err
	}
	to, err := b.compileVamExprWithEmpty(slice.To)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewSliceExpr(b.sctx(), e, from, to), nil
}

func (b *Builder) compileVamArrayExpr(e *dag.ArrayExpr) (vamexpr.Evaluator, error) {
	elems, err := b.compileVamListElems(e.Elems)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewArrayExpr(b.sctx(), elems), nil
}

func (b *Builder) compileVamSetExpr(e *dag.SetExpr) (vamexpr.Evaluator, error) {
	elems, err := b.compileVamListElems(e.Elems)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewSetExpr(b.sctx(), elems), nil
}

func (b *Builder) compileVamListElems(elems []dag.VectorElem) ([]vamexpr.ListElem, error) {
	var out []vamexpr.ListElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *dag.Spread:
			e, err := b.compileVamExpr(elem.Expr)
			if err != nil {
				return nil, err
			}
			out = append(out, vamexpr.ListElem{Spread: e})
		case *dag.VectorValue:
			e, err := b.compileVamExpr(elem.Expr)
			if err != nil {
				return nil, err
			}
			out = append(out, vamexpr.ListElem{Value: e})
		default:
			panic(elem)
		}
	}
	return out, nil
}
