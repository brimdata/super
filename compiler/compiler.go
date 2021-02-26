package compiler

import (
	"github.com/brimsec/zq/compiler/ast"
	"github.com/brimsec/zq/compiler/kernel"
	"github.com/brimsec/zq/compiler/semantic"
	"github.com/brimsec/zq/expr"
	"github.com/brimsec/zq/field"
	"github.com/brimsec/zq/proc"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/brimsec/zq/zql"
)

// ParseProc() is an entry point for use from external go code,
// mostly just a wrapper around Parse() that casts the return value.
func ParseProc(z string) (ast.Proc, error) {
	parsed, err := zql.ParseZ(z)
	if err != nil {
		return nil, err
	}
	return ast.UnpackMapAsProc(parsed)
}

func ParseExpression(expr string) (ast.Expression, error) {
	m, err := zql.ParseZByRule("Expr", expr)
	if err != nil {
		return nil, err
	}
	return ast.UnpackMapAsExpr(m)
}

// MustParseProc is functionally the same as ParseProc but panics if an error
// is encountered.
func MustParseProc(query string) ast.Proc {
	proc, err := ParseProc(query)
	if err != nil {
		panic(err)
	}
	return proc
}

// XXX These functions will all get reworked in a subsequent PR when
// the semantic pass converts an AST to a flow DSL.

func Optimize(zctx *resolver.Context, program ast.Proc, sortKey field.Static, sortReversed bool) (*kernel.Filter, ast.Proc) {
	return semantic.Optimize(zctx, program, sortKey, sortReversed)
}

func IsParallelizable(p ast.Proc, inputSortField field.Static, inputSortReversed bool) bool {
	return semantic.IsParallelizable(p, inputSortField, inputSortReversed)
}

func Parallelize(p ast.Proc, N int, inputSortField field.Static, inputSortReversed bool) (*ast.SequentialProc, bool) {
	return semantic.Parallelize(p, N, inputSortField, inputSortReversed)
}

func NewFilter(zctx *resolver.Context, ast ast.Expression) *kernel.Filter {
	return kernel.NewFilter(zctx, ast)
}
func Compile(custom kernel.Hook, node ast.Proc, pctx *proc.Context, parents []proc.Interface) ([]proc.Interface, error) {
	return kernel.Compile(custom, node, pctx, nil, parents)
}

func CompileAssignments(dsts []field.Static, srcs []field.Static) ([]field.Static, []expr.Evaluator) {
	return kernel.CompileAssignments(dsts, srcs)
}
