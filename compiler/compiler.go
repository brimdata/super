package compiler

import (
	"github.com/brimdata/zed/compiler/ast"
	"github.com/brimdata/zed/compiler/ast/dag"
	"github.com/brimdata/zed/compiler/kernel"
	"github.com/brimdata/zed/compiler/optimizer"
	"github.com/brimdata/zed/compiler/parser"
	"github.com/brimdata/zed/compiler/semantic"
	"github.com/brimdata/zed/expr"
	"github.com/brimdata/zed/field"
	"github.com/brimdata/zed/proc"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zson"
)

var _ zbuf.Filter = (*Runtime)(nil)

type Runtime struct {
	zctx      *zson.Context
	scope     *kernel.Scope
	optimizer *optimizer.Optimizer
	consts    []dag.Op
	outputs   []proc.Interface
}

func New(zctx *zson.Context, parserAST ast.Proc) (*Runtime, error) {
	return NewWithSortedInput(zctx, parserAST, nil, false)
}

func NewWithZ(zctx *zson.Context, z string) (*Runtime, error) {
	p, err := ParseProc(z)
	if err != nil {
		return nil, err
	}
	return New(zctx, p)
}

func NewWithSortedInput(zctx *zson.Context, parserAST ast.Proc, sortKey field.Static, sortRev bool) (*Runtime, error) {
	op, consts, err := semantic.Analyze(parserAST)
	if err != nil {
		return nil, err
	}
	opt := optimizer.New(op)
	if sortKey != nil {
		opt.SetInputOrder(sortKey, sortRev)
	}
	scope := kernel.NewScope()
	// enter the global scope
	scope.Enter()
	if err := kernel.LoadConsts(zctx, scope, consts); err != nil {
		return nil, err
	}
	return &Runtime{
		zctx:      zctx,
		scope:     scope,
		optimizer: opt,
		consts:    consts,
	}, nil
}

func (r *Runtime) Outputs() []proc.Interface {
	return r.outputs
}

func (r *Runtime) Entry() dag.Op {
	//XXX need to prepend consts depending on context
	return r.optimizer.Entry()
}

func (r *Runtime) AsFilter() (expr.Filter, error) {
	if r == nil {
		return nil, nil
	}
	f := r.optimizer.Filter()
	if f == nil {
		return nil, nil
	}
	return kernel.CompileFilter(r.zctx, r.scope, f)
}

func (r *Runtime) AsBufferFilter() (*expr.BufferFilter, error) {
	if r == nil {
		return nil, nil
	}
	f := r.optimizer.Filter()
	if f == nil {
		return nil, nil
	}
	return kernel.CompileBufferFilter(f)
}

// AsProc returns the lifted filter and any consts if present as a proc so that,
// for instance, the root worker (or a sub-worker) can push the filter over the
// net to the source scanner.
func (r *Runtime) AsOp() dag.Op {
	if r == nil {
		return nil
	}
	f := r.optimizer.Filter()
	if f == nil {
		return nil
	}
	filterOp := &dag.Filter{
		Kind: "Filter",
		Expr: f,
	}
	consts := r.consts
	if len(consts) == 0 {
		return filterOp
	}
	ops := make([]dag.Op, 0, len(consts)+1)
	ops = append(ops, consts...)
	ops = append(ops, filterOp)
	return &dag.Sequential{
		Kind: "Sequential",
		Ops:  ops,
	}
}

// This must be called before the zbuf.Filter interface will work.
func (r *Runtime) Optimize() error {
	return r.optimizer.Optimize()
}

func (r *Runtime) IsParallelizable() bool {
	return r.optimizer.IsParallelizable()
}

func (r *Runtime) Parallelize(n int) bool {
	return r.optimizer.Parallelize(n)
}

// ParseProc() is an entry point for use from external go code,
// mostly just a wrapper around Parse() that casts the return value.
func ParseProc(z string) (ast.Proc, error) {
	parsed, err := parser.ParseZ(z)
	if err != nil {
		return nil, err
	}
	return ast.UnpackMapAsProc(parsed)
}

func ParseExpression(expr string) (ast.Expr, error) {
	m, err := parser.ParseZByRule("Expr", expr)
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

func (r *Runtime) Compile(custom kernel.Hook, pctx *proc.Context, inputs []proc.Interface) error {
	var err error
	r.outputs, err = kernel.Compile(custom, r.optimizer.Entry(), pctx, r.scope, inputs)
	return err
}

func CompileAssignments(dsts []field.Static, srcs []field.Static) ([]field.Static, []expr.Evaluator) {
	return kernel.CompileAssignments(dsts, srcs)
}

func CompileProc(p ast.Proc, pctx *proc.Context, inputs []proc.Interface) (*Runtime, error) {
	r, err := New(pctx.Zctx, p)
	if err != nil {
		return nil, err
	}
	if err := r.Compile(nil, pctx, inputs); err != nil {
		return nil, err
	}
	return r, nil
}

func CompileZ(z string, pctx *proc.Context, inputs []proc.Interface) ([]proc.Interface, error) {
	p, err := ParseProc(z)
	if err != nil {
		return nil, err
	}
	runtime, err := CompileProc(p, pctx, inputs)
	if err != nil {
		return nil, err
	}
	return runtime.Outputs(), nil
}
