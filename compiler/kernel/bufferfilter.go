package kernel

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sup"
	"golang.org/x/text/unicode/norm"
)

// CompileBufferFilter tries to return a BufferFilter for e such that the
// BufferFilter's Eval method returns true for any byte slice containing the ZNG
// encoding of a record matching e. (It may also return true for some byte
// slices that do not match.) compileBufferFilter returns a nil pointer and nil
// error if it cannot construct a useful filter.
func CompileBufferFilter(zctx *super.Context, e dag.Expr) (*expr.BufferFilter, error) {
	switch e := e.(type) {
	case *dag.BinaryExpr:
		literal, err := isFieldEqualOrIn(zctx, e)
		if err != nil {
			return nil, err
		}
		if literal != nil {
			return newBufferFilterForLiteral(*literal)
		}
		if e.Op == "and" {
			left, err := CompileBufferFilter(zctx, e.LHS)
			if err != nil {
				return nil, err
			}
			right, err := CompileBufferFilter(zctx, e.RHS)
			if err != nil {
				return nil, err
			}
			if left == nil {
				return right, nil
			}
			if right == nil {
				return left, nil
			}
			return expr.NewAndBufferFilter(left, right), nil
		}
		if e.Op == "or" {
			left, err := CompileBufferFilter(zctx, e.LHS)
			if err != nil {
				return nil, err
			}
			right, err := CompileBufferFilter(zctx, e.RHS)
			if left == nil || right == nil || err != nil {
				return nil, err
			}
			return expr.NewOrBufferFilter(left, right), nil
		}
		return nil, nil
	case *dag.Search:
		literal, err := sup.ParseValue(zctx, e.Value)
		if err != nil {
			return nil, err
		}
		switch super.TypeUnder(literal.Type()) {
		case super.TypeNet:
			return nil, nil
		case super.TypeString:
			pattern := norm.NFC.Bytes(literal.Bytes())
			left := expr.NewBufferFilterForStringCase(string(pattern))
			if left == nil {
				return nil, nil
			}
			right := expr.NewBufferFilterForFieldName(string(pattern))
			return expr.NewOrBufferFilter(left, right), nil
		}
		left := expr.NewBufferFilterForStringCase(e.Text)
		right, err := newBufferFilterForLiteral(literal)
		if left == nil || right == nil || err != nil {
			return nil, err
		}
		return expr.NewOrBufferFilter(left, right), nil
	default:
		return nil, nil
	}
}

func isFieldEqualOrIn(zctx *super.Context, e *dag.BinaryExpr) (*super.Value, error) {
	if _, ok := e.LHS.(*dag.This); ok && e.Op == "==" {
		if literal, ok := e.RHS.(*dag.Literal); ok {
			val, err := sup.ParseValue(zctx, literal.Value)
			if err != nil {
				return nil, err
			}
			return &val, nil
		}
	} else if _, ok := e.RHS.(*dag.This); ok && e.Op == "in" {
		if literal, ok := e.LHS.(*dag.Literal); ok {
			val, err := sup.ParseValue(zctx, literal.Value)
			if err != nil {
				return nil, err
			}
			if val.Type() == super.TypeNet {
				return nil, err
			}
			return &val, nil
		}
	}
	return nil, nil
}

func newBufferFilterForLiteral(val super.Value) (*expr.BufferFilter, error) {
	if id := val.Type().ID(); super.IsNumber(id) || id == super.IDNull {
		// All numbers are comparable, so they can require up to three
		// patterns: float, varint, and uvarint.
		return nil, nil
	}
	// We're looking for a complete ZNG value, so we can lengthen the
	// pattern by calling Encode to add a tag.
	pattern := string(val.Encode(nil))
	return expr.NewBufferFilterForString(pattern), nil
}
