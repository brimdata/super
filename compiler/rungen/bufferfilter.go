package rungen

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sup"
	"golang.org/x/text/unicode/norm"
)

// CompileBufferFilter tries to return a BufferFilter for e such that the
// BufferFilter's Eval method returns true for any byte slice containing the BSUP
// encoding of a record matching e. (It may also return true for some byte
// slices that do not match.) compileBufferFilter returns a nil pointer and nil
// error if it cannot construct a useful filter.
func CompileBufferFilter(sctx *super.Context, e dag.Expr) (*expr.BufferFilter, error) {
	switch e := e.(type) {
	case *dag.BinaryExpr:
		literal, err := isFieldEqualOrIn(sctx, e)
		if err != nil {
			return nil, err
		}
		if literal != nil {
			return newBufferFilterForLiteral(*literal)
		}
		if e.Op == "and" {
			left, err := CompileBufferFilter(sctx, e.LHS)
			if err != nil {
				return nil, err
			}
			right, err := CompileBufferFilter(sctx, e.RHS)
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
			left, err := CompileBufferFilter(sctx, e.LHS)
			if err != nil {
				return nil, err
			}
			right, err := CompileBufferFilter(sctx, e.RHS)
			if left == nil || right == nil || err != nil {
				return nil, err
			}
			return expr.NewOrBufferFilter(left, right), nil
		}
		return nil, nil
	case *dag.SearchExpr:
		literal, err := sup.ParseValue(sctx, e.Value)
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

func isFieldEqualOrIn(sctx *super.Context, e *dag.BinaryExpr) (*super.Value, error) {
	if _, ok := e.LHS.(*dag.ThisExpr); ok && e.Op == "==" {
		if literal, ok := e.RHS.(*dag.LiteralExpr); ok {
			val, err := sup.ParseValue(sctx, literal.Value)
			if err != nil {
				return nil, err
			}
			return &val, nil
		}
	} else if _, ok := e.RHS.(*dag.ThisExpr); ok && e.Op == "in" {
		if literal, ok := e.LHS.(*dag.LiteralExpr); ok {
			val, err := sup.ParseValue(sctx, literal.Value)
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
	// We're looking for a complete BSUP value, so we can lengthen the
	// pattern by calling Encode to add a tag.
	pattern := string(val.Encode(nil))
	return expr.NewBufferFilterForString(pattern), nil
}
