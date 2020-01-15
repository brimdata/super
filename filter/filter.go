package filter

import (
	"fmt"

	"github.com/mccanne/zq/ast"
	"github.com/mccanne/zq/expr"
	"github.com/mccanne/zq/zcode"
	"github.com/mccanne/zq/zng"
	"github.com/mccanne/zq/zx"
)

type Filter func(*zng.Record) bool

func LogicalAnd(left, right Filter) Filter {
	return func(p *zng.Record) bool { return left(p) && right(p) }
}

func LogicalOr(left, right Filter) Filter {
	return func(p *zng.Record) bool { return left(p) || right(p) }
}

func LogicalNot(expr Filter) Filter {
	return func(p *zng.Record) bool { return !expr(p) }
}

func combine(res expr.FieldExprResolver, pred zx.Predicate) Filter {
	return func(r *zng.Record) bool {
		v := res(r)
		if v.Type == nil {
			// field (or sub-field) doesn't exist in this record
			return false
		}
		return pred(v)
	}
}

func CompileFieldCompare(node *ast.CompareField) (Filter, error) {
	literal := node.Value
	// Treat len(field) specially since we're looking at a computed
	// value rather than a field from a record.

	// XXX we need to implement proper expressions
	if op, ok := node.Field.(*ast.FieldCall); ok && op.Fn == "Len" {
		i, err := zng.AsInt64(literal)
		if err != nil {
			return nil, fmt.Errorf("cannot compare len() with non-integer: %s", err)
		}
		comparison, err := zx.CompareContainerLen(node.Comparator, i)
		resolver, err := expr.CompileFieldExpr(op.Field)
		if err != nil {
			return nil, err
		}
		return combine(resolver, comparison), nil
	}

	comparison, err := zx.Comparison(node.Comparator, literal)
	if err != nil {
		return nil, err
	}
	resolver, err := expr.CompileFieldExpr(node.Field)
	if err != nil {
		return nil, err
	}
	return combine(resolver, comparison), nil
}

func EvalAny(eval zx.Predicate, recursive bool) Filter {
	if !recursive {
		return func(r *zng.Record) bool {
			it := r.ZvalIter()
			for _, c := range r.Type.Columns {
				val, _, err := it.Next()
				if err != nil {
					return false
				}
				if eval(zng.Value{c.Type, val}) {
					return true
				}
			}
			return false
		}
	}

	var fn func(v zcode.Bytes, cols []zng.Column) bool
	fn = func(v zcode.Bytes, cols []zng.Column) bool {
		it := zcode.Iter(v)
		for _, c := range cols {
			val, _, err := it.Next()
			if err != nil {
				return false
			}
			recType, isRecord := c.Type.(*zng.TypeRecord)
			if isRecord && fn(val, recType.Columns) {
				return true
			} else if !isRecord && eval(zng.Value{c.Type, val}) {
				return true
			}
		}
		return false
	}
	return func(r *zng.Record) bool {
		return fn(r.Raw, r.Type.Columns)
	}
}

func Compile(node ast.BooleanExpr) (Filter, error) {
	switch v := node.(type) {
	case *ast.LogicalNot:
		expr, err := Compile(v.Expr)
		if err != nil {
			return nil, err
		}
		return LogicalNot(expr), nil

	case *ast.LogicalAnd:
		left, err := Compile(v.Left)
		if err != nil {
			return nil, err
		}
		right, err := Compile(v.Right)
		if err != nil {
			return nil, err
		}
		return LogicalAnd(left, right), nil

	case *ast.LogicalOr:
		left, err := Compile(v.Left)
		if err != nil {
			return nil, err
		}
		right, err := Compile(v.Right)
		if err != nil {
			return nil, err
		}
		return LogicalOr(left, right), nil

	case *ast.BooleanLiteral:
		return func(*zng.Record) bool { return v.Value }, nil

	case *ast.CompareField:
		if v.Comparator == "in" {
			resolver, err := expr.CompileFieldExpr(v.Field)
			if err != nil {
				return nil, err
			}
			eql, _ := zx.Comparison("eql", v.Value)
			comparison := zx.Contains(eql)
			return combine(resolver, comparison), nil
		}

		return CompileFieldCompare(v)

	case *ast.CompareAny:
		if v.Comparator == "in" {
			compare, err := zx.Comparison("eql", v.Value)
			if err != nil {
				return nil, err
			}
			contains := zx.Contains(compare)
			return EvalAny(contains, v.Recursive), nil
		}
		//XXX this is messed up
		if v.Comparator == "searchin" {
			search, err := zx.Comparison("search", v.Value)
			if err != nil {
				return nil, err
			}
			contains := zx.Contains(search)
			return EvalAny(contains, v.Recursive), nil
		}

		comparison, err := zx.Comparison(v.Comparator, v.Value)
		if err != nil {
			return nil, err
		}
		return EvalAny(comparison, v.Recursive), nil

	default:
		return nil, fmt.Errorf("Filter AST unknown type: %v", v)
	}
}
