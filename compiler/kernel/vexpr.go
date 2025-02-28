package kernel

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
	"github.com/brimdata/super/zson"
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
		val, err := zson.ParseValue(b.zctx(), e.Value)
		if err != nil {
			return nil, err
		}
		return vamexpr.NewLiteral(val), nil
	//case *dag.Var:
	//	return vamexpr.NewVar(e.Slot), nil
	case *dag.Search:
		return b.compileVamSearch(e)
	case *dag.This:
		return vamexpr.NewDottedExpr(b.zctx(), field.Path(e.Path)), nil
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
	case *dag.RegexpMatch:
		return b.compileVamRegexpMatch(e)
	case *dag.RegexpSearch:
		return b.compileVamRegexpSearch(e)
	case *dag.RecordExpr:
		return b.compileVamRecordExpr(e)
	case *dag.SliceExpr:
		return b.compileVamSliceExpr(e)
	//case *dag.SetExpr:
	//	return b.compileVamSetExpr(e)
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
		return vamexpr.NewLogicalAnd(b.zctx(), lhs, rhs), nil
	case "or":
		return vamexpr.NewLogicalOr(b.zctx(), lhs, rhs), nil
	case "in":
		return vamexpr.NewIn(b.zctx(), lhs, rhs), nil
	case "==", "!=", "<", "<=", ">", ">=":
		return vamexpr.NewCompare(b.zctx(), lhs, rhs, op), nil
	case "+", "-", "*", "/", "%":
		return vamexpr.NewArith(b.zctx(), lhs, rhs, op), nil
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
	return vamexpr.NewConditional(b.zctx(), predicate, thenExpr, elseExpr), nil
}

func (b *Builder) compileVamUnary(unary dag.UnaryExpr) (vamexpr.Evaluator, error) {
	e, err := b.compileVamExpr(unary.Operand)
	if err != nil {
		return nil, err
	}
	switch unary.Op {
	case "-":
		return vamexpr.NewUnaryMinus(b.zctx(), e), nil
	case "!":
		return vamexpr.NewLogicalNot(b.zctx(), e), nil
	default:
		return nil, fmt.Errorf("unknown unary operator %s", unary.Op)
	}
}

func (b *Builder) compileVamDotExpr(dot *dag.Dot) (vamexpr.Evaluator, error) {
	record, err := b.compileVamExpr(dot.LHS)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewDotExpr(b.zctx(), record, dot.RHS), nil
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
	return vamexpr.NewIndexExpr(b.zctx(), e, index), nil
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
	if call.Name == "cast" {
		return b.compileVamCast(call.Args)
	}
	fn, path, err := vamfunction.New(b.zctx(), call.Name, len(call.Args))
	if err != nil {
		return nil, err
	}
	args := call.Args
	if path != nil {
		dagPath := &dag.This{Kind: "This", Path: path}
		args = append([]dag.Expr{dagPath}, args...)
	}
	exprs, err := b.compileVamExprs(args)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewCall(fn, exprs), nil
}

func (b *Builder) compileVamCast(args []dag.Expr) (vamexpr.Evaluator, error) {
	if err := function.CheckArgCount(len(args), 2, 2); err != nil {
		return nil, err
	}
	exprs, err := b.compileVamExprs(args)
	if err != nil {
		return nil, err
	}
	literal, ok := exprs[1].(*vamexpr.Literal)
	if !ok {
		return nil, errors.New("cast: vector runtime only supports casts on constant values at this time")
	}
	return vamexpr.NewLiteralCast(b.zctx(), exprs[0], literal)
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
	return vamexpr.NewRecordExpr(b.zctx(), elems), nil
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
	val, err := zson.ParseValue(b.zctx(), search.Value)
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
	return vamexpr.NewSliceExpr(b.zctx(), e, from, to), nil
}

func (b *Builder) compileVamArrayExpr(e *dag.ArrayExpr) (vamexpr.Evaluator, error) {
	elems, err := b.compileVamListElems(e.Elems)
	if err != nil {
		return nil, err
	}
	return vamexpr.NewArrayExpr(b.zctx(), elems), nil
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
