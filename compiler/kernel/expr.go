package kernel

import (
	"errors"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/runtime/sam/op/combine"
	"github.com/brimdata/super/runtime/sam/op/traverse"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zson"
	"golang.org/x/text/unicode/norm"
)

// compileExpr compiles the given Expression into an object
// that evaluates the expression against a provided Record.  It returns an
// error if compilation fails for any reason.
//
// This is the "intepreted slow path" of the analytics engine.  Because it
// handles dynamic typing at runtime, overheads are incurred due to
// various type checks and coercions that determine different computational
// outcomes based on type.  There is nothing here that optimizes analytics
// for native machine types; these optimizations (will) happen in the pushdown
// predicate processing engine in the VNG columnar scanner.
//
// Eventually, we will optimize this VNG "fast path" by dynamically
// generating byte codes (which can in turn be JIT assembled into machine code)
// for each zng TypeRecord encountered.  Once you know the TypeRecord,
// you can generate code using strong typing just as an OLAP system does
// due to its schemas defined up-front in its relational tables.  Here,
// each record type is like a schema and as we encounter them, we can compile
// optimized code for the now-static types within that record type.
//
// The Evaluator return by CompileExpr produces super.Values that are stored
// in temporary buffers and may be modified on subsequent calls to Eval.
// This is intended to minimize the garbage collection needs of the inner loop
// by not allocating memory on a per-Eval basis.  For uses like filtering and
// aggregations, where the results are immediately used, this is desirable and
// efficient but for use cases like storing the results as groupby keys, the
// resulting super.Value should be copied (e.g., via super.Value.Copy()).
//
// TBD: string values and net.IP address do not need to be copied because they
// are allocated by go libraries and temporary buffers are not used.  This will
// change down the road when we implement no-allocation string and IP conversion.
func (b *Builder) compileExpr(e dag.Expr) (expr.Evaluator, error) {
	if e == nil {
		return nil, errors.New("null expression not allowed")
	}
	switch e := e.(type) {
	case *dag.Literal:
		val, err := zson.ParseValue(b.zctx(), e.Value)
		if err != nil {
			return nil, err
		}
		return expr.NewLiteral(val), nil
	case *dag.Var:
		return expr.NewVar(e.Slot), nil
	case *dag.Search:
		return b.compileSearch(e)
	case *dag.This:
		return expr.NewDottedExpr(b.zctx(), field.Path(e.Path)), nil
	case *dag.Dot:
		return b.compileDotExpr(e)
	case *dag.UnaryExpr:
		return b.compileUnary(*e)
	case *dag.BinaryExpr:
		return b.compileBinary(e)
	case *dag.Conditional:
		return b.compileConditional(*e)
	case *dag.Call:
		return b.compileCall(*e)
	case *dag.IndexExpr:
		return b.compileIndexExpr(e)
	case *dag.IsNullExpr:
		return b.compileIsNullExpr(e)
	case *dag.SliceExpr:
		return b.compileSliceExpr(e)
	case *dag.RegexpMatch:
		return b.compileRegexpMatch(e)
	case *dag.RegexpSearch:
		return b.compileRegexpSearch(e)
	case *dag.RecordExpr:
		return b.compileRecordExpr(e)
	case *dag.ArrayExpr:
		return b.compileArrayExpr(e)
	case *dag.SetExpr:
		return b.compileSetExpr(e)
	case *dag.MapCall:
		return b.compileMapCall(e)
	case *dag.MapExpr:
		return b.compileMapExpr(e)
	case *dag.Agg:
		agg, err := b.compileAgg(e)
		if err != nil {
			return nil, err
		}
		aggexpr := expr.NewAggregatorExpr(b.zctx(), agg)
		b.resetters = append(b.resetters, aggexpr)
		return aggexpr, nil
	case *dag.OverExpr:
		return b.compileOverExpr(e)
	default:
		return nil, fmt.Errorf("invalid expression type %T", e)
	}
}

func (b *Builder) compileExprWithEmpty(e dag.Expr) (expr.Evaluator, error) {
	if e == nil {
		return nil, nil
	}
	return b.compileExpr(e)
}

func (b *Builder) compileBinary(e *dag.BinaryExpr) (expr.Evaluator, error) {
	if e.Op == "in" {
		// Do a faster comparison if the LHS is a compile-time constant expression.
		if in, err := b.compileConstIn(e); in != nil && err == nil {
			return in, err
		}
	}
	if e, err := b.compileConstCompare(e); e != nil && err == nil {
		return e, nil
	}
	lhs, err := b.compileExpr(e.LHS)
	if err != nil {
		return nil, err
	}
	rhs, err := b.compileExpr(e.RHS)
	if err != nil {
		return nil, err
	}
	switch op := e.Op; op {
	case "and":
		return expr.NewLogicalAnd(b.zctx(), lhs, rhs), nil
	case "or":
		return expr.NewLogicalOr(b.zctx(), lhs, rhs), nil
	case "in":
		return expr.NewIn(b.zctx(), lhs, rhs), nil
	case "==", "!=":
		return expr.NewCompareEquality(b.zctx(), lhs, rhs, op)
	case "<", "<=", ">", ">=":
		return expr.NewCompareRelative(b.zctx(), lhs, rhs, op)
	case "+", "-", "*", "/", "%":
		return expr.NewArithmetic(b.zctx(), lhs, rhs, op)
	default:
		return nil, fmt.Errorf("invalid binary operator %s", op)
	}
}

func (b *Builder) compileConstIn(e *dag.BinaryExpr) (expr.Evaluator, error) {
	literal, err := b.evalAtCompileTime(e.LHS)
	if err != nil || literal.IsError() {
		// If the RHS here is a literal value, it would be good
		// to optimize this too.  However, this will all be handled
		// by the JIT compiler that will create optimized closures
		// on a per-type basis.
		return nil, nil
	}
	eql, err := expr.Comparison("==", literal)
	if eql == nil || err != nil {
		return nil, nil
	}
	operand, err := b.compileExpr(e.RHS)
	if err != nil {
		return nil, err
	}
	return expr.NewFilter(operand, expr.Contains(eql)), nil
}

func (b *Builder) compileConstCompare(e *dag.BinaryExpr) (expr.Evaluator, error) {
	switch e.Op {
	case "==", "!=", "<", "<=", ">", ">=":
	default:
		return nil, nil
	}
	literal, err := b.evalAtCompileTime(e.RHS)
	if err != nil || literal.IsError() {
		return nil, nil
	}
	comparison, err := expr.Comparison(e.Op, literal)
	if comparison == nil || err != nil {
		// If this fails, return no match instead of the error and
		// let later-on code detect the error as this could be a
		// non-error situation that isn't a simple comparison.
		return nil, nil
	}
	operand, err := b.compileExpr(e.LHS)
	if err != nil {
		return nil, err
	}
	return expr.NewFilter(operand, comparison), nil
}

func (b *Builder) compileSearch(search *dag.Search) (expr.Evaluator, error) {
	val, err := zson.ParseValue(b.zctx(), search.Value)
	if err != nil {
		return nil, err
	}
	e, err := b.compileExpr(search.Expr)
	if err != nil {
		return nil, err
	}
	if super.TypeUnder(val.Type()) == super.TypeString {
		// Do a grep-style substring search instead of an
		// exact match on each value.
		term := norm.NFC.Bytes(val.Bytes())
		return expr.NewSearchString(string(term), e), nil
	}
	return expr.NewSearch(search.Text, val, e)
}

func (b *Builder) compileSliceExpr(slice *dag.SliceExpr) (expr.Evaluator, error) {
	e, err := b.compileExpr(slice.Expr)
	if err != nil {
		return nil, err
	}
	from, err := b.compileExprWithEmpty(slice.From)
	if err != nil {
		return nil, err
	}
	to, err := b.compileExprWithEmpty(slice.To)
	if err != nil {
		return nil, err
	}
	return expr.NewSlice(b.zctx(), e, from, to), nil
}

func (b *Builder) compileUnary(unary dag.UnaryExpr) (expr.Evaluator, error) {
	e, err := b.compileExpr(unary.Operand)
	if err != nil {
		return nil, err
	}
	switch unary.Op {
	case "-":
		return expr.NewUnaryMinus(b.zctx(), e), nil
	case "!":
		return expr.NewLogicalNot(b.zctx(), e), nil
	default:
		return nil, fmt.Errorf("unknown unary operator %s", unary.Op)
	}
}

func (b *Builder) compileConditional(node dag.Conditional) (expr.Evaluator, error) {
	predicate, err := b.compileExpr(node.Cond)
	if err != nil {
		return nil, err
	}
	thenExpr, err := b.compileExpr(node.Then)
	if err != nil {
		return nil, err
	}
	elseExpr, err := b.compileExpr(node.Else)
	if err != nil {
		return nil, err
	}
	return expr.NewConditional(b.zctx(), predicate, thenExpr, elseExpr), nil
}

func (b *Builder) compileDotExpr(dot *dag.Dot) (expr.Evaluator, error) {
	record, err := b.compileExpr(dot.LHS)
	if err != nil {
		return nil, err
	}
	return expr.NewDotExpr(b.zctx(), record, dot.RHS), nil
}

func (b *Builder) compileLval(e dag.Expr) (*expr.Lval, error) {
	switch e := e.(type) {
	case *dag.IndexExpr:
		container, err := b.compileLval(e.Expr)
		if err != nil {
			return nil, err
		}
		index, err := b.compileExpr(e.Index)
		if err != nil {
			return nil, err
		}
		container.Elems = append(container.Elems, expr.NewExprLvalElem(b.zctx(), index))
		return container, nil
	case *dag.Dot:
		lhs, err := b.compileLval(e.LHS)
		if err != nil {
			return nil, err
		}
		lhs.Elems = append(lhs.Elems, &expr.StaticLvalElem{Name: e.RHS})
		return lhs, nil
	case *dag.This:
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

func (b *Builder) compileCall(call dag.Call) (expr.Evaluator, error) {
	if tf := expr.NewShaperTransform(call.Name); tf != 0 {
		return b.compileShaper(call.Args, tf)
	}
	var path field.Path
	// First check if call is to a user defined function, otherwise check for
	// builtin function.
	var fn expr.Function
	if e, ok := b.udfs[call.Name]; ok {
		var err error
		if fn, err = b.compileUDFCall(call.Name, e); err != nil {
			return nil, err
		}
	} else {
		var err error
		fn, path, err = function.New(b.zctx(), call.Name, len(call.Args))
		if err != nil {
			return nil, fmt.Errorf("%s(): %w", call.Name, err)
		}
	}
	args := call.Args
	if path != nil {
		dagPath := &dag.This{Kind: "This", Path: path}
		args = append([]dag.Expr{dagPath}, args...)
	}
	exprs, err := b.compileExprs(args)
	if err != nil {
		return nil, fmt.Errorf("%s(): bad argument: %w", call.Name, err)
	}
	return expr.NewCall(fn, exprs), nil
}

func (b *Builder) compileUDFCall(name string, body dag.Expr) (expr.Function, error) {
	if fn, ok := b.compiledUDFs[name]; ok {
		return fn, nil
	}
	fn := &expr.UDF{Name: name, Zctx: b.zctx()}
	// We store compiled UDF calls here so as to avoid stack overflows on
	// recursive calls.
	b.compiledUDFs[name] = fn
	var err error
	if fn.Body, err = b.compileExpr(body); err != nil {
		return nil, err
	}
	delete(b.compiledUDFs, name)
	return fn, nil
}

func (b *Builder) compileMapCall(a *dag.MapCall) (expr.Evaluator, error) {
	e, err := b.compileExpr(a.Expr)
	if err != nil {
		return nil, err
	}
	inner, err := b.compileExpr(a.Inner)
	if err != nil {
		return nil, err
	}
	return expr.NewMapCall(b.zctx(), e, inner), nil
}

func (b *Builder) compileShaper(args []dag.Expr, tf expr.ShaperTransform) (expr.Evaluator, error) {
	field, err := b.compileExpr(args[0])
	if err != nil {
		return nil, err
	}
	typExpr, err := b.compileExpr(args[1])
	if err != nil {
		return nil, err
	}
	return expr.NewShaper(b.zctx(), field, typExpr, tf)
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

func (b *Builder) compileIndexExpr(e *dag.IndexExpr) (expr.Evaluator, error) {
	container, err := b.compileExpr(e.Expr)
	if err != nil {
		return nil, err
	}
	index, err := b.compileExpr(e.Index)
	if err != nil {
		return nil, err
	}
	return expr.NewIndexExpr(b.zctx(), container, index), nil
}

func (b *Builder) compileIsNullExpr(e *dag.IsNullExpr) (expr.Evaluator, error) {
	eval, err := b.compileExpr(e.Expr)
	if err != nil {
		return nil, err
	}
	return expr.NewIsNullExpr(eval), nil
}

func (b *Builder) compileRegexpMatch(match *dag.RegexpMatch) (expr.Evaluator, error) {
	e, err := b.compileExpr(match.Expr)
	if err != nil {
		return nil, err
	}
	re, err := expr.CompileRegexp(match.Pattern)
	if err != nil {
		return nil, err
	}
	return expr.NewRegexpMatch(re, e), nil
}

func (b *Builder) compileRegexpSearch(search *dag.RegexpSearch) (expr.Evaluator, error) {
	e, err := b.compileExpr(search.Expr)
	if err != nil {
		return nil, err
	}
	re, err := expr.CompileRegexp(search.Pattern)
	if err != nil {
		return nil, err
	}
	match := expr.NewRegexpBoolean(re)
	return expr.SearchByPredicate(expr.Contains(match), e), nil
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
			elems = append(elems, expr.RecordElem{
				Name:  elem.Name,
				Field: e,
			})
		case *dag.Spread:
			e, err := b.compileExpr(elem.Expr)
			if err != nil {
				return nil, err
			}
			elems = append(elems, expr.RecordElem{Spread: e})
		}
	}
	return expr.NewRecordExpr(b.zctx(), elems)
}

func (b *Builder) compileArrayExpr(array *dag.ArrayExpr) (expr.Evaluator, error) {
	elems, err := b.compileVectorElems(array.Elems)
	if err != nil {
		return nil, err
	}
	return expr.NewArrayExpr(b.zctx(), elems), nil
}

func (b *Builder) compileSetExpr(set *dag.SetExpr) (expr.Evaluator, error) {
	elems, err := b.compileVectorElems(set.Elems)
	if err != nil {
		return nil, err
	}
	return expr.NewSetExpr(b.zctx(), elems), nil
}

func (b *Builder) compileVectorElems(elems []dag.VectorElem) ([]expr.VectorElem, error) {
	var out []expr.VectorElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *dag.Spread:
			e, err := b.compileExpr(elem.Expr)
			if err != nil {
				return nil, err
			}
			out = append(out, expr.VectorElem{Spread: e})
		case *dag.VectorValue:
			e, err := b.compileExpr(elem.Expr)
			if err != nil {
				return nil, err
			}
			out = append(out, expr.VectorElem{Value: e})
		}
	}
	return out, nil
}

func (b *Builder) compileMapExpr(m *dag.MapExpr) (expr.Evaluator, error) {
	var entries []expr.Entry
	for _, f := range m.Entries {
		key, err := b.compileExpr(f.Key)
		if err != nil {
			return nil, err
		}
		val, err := b.compileExpr(f.Value)
		if err != nil {
			return nil, err
		}
		entries = append(entries, expr.Entry{Key: key, Val: val})
	}
	return expr.NewMapExpr(b.zctx(), entries), nil
}

func (b *Builder) compileOverExpr(over *dag.OverExpr) (expr.Evaluator, error) {
	names, lets, err := b.compileDefs(over.Defs)
	if err != nil {
		return nil, err
	}
	exprs, err := b.compileExprs(over.Exprs)
	if err != nil {
		return nil, err
	}
	parent := traverse.NewExpr(b.rctx.Context, b.zctx())
	enter := traverse.NewOver(b.rctx, parent, exprs, expr.Resetters{})
	scope := enter.AddScope(b.rctx.Context, names, lets)
	exits, err := b.compileSeq(over.Body, []zbuf.Puller{scope})
	if err != nil {
		return nil, err
	}
	var exit zbuf.Puller
	if len(exits) == 1 {
		exit = exits[0]
	} else {
		// This can happen when output of over body
		// is a fork or switch.
		exit = combine.New(b.rctx, exits)
	}
	parent.SetExit(scope.NewExit(exit))
	return parent, nil
}
