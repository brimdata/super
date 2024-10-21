package semantic

import (
	"context"
	"errors"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/ast/dag"
	"github.com/brimdata/super/compiler/data"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/lakeparse"
)

// Analyze performs a semantic analysis of the AST, translating it from AST
// to DAG form, resolving syntax ambiguities, and performing constant propagation.
// After semantic analysis, the DAG is ready for either optimization or compilation.
func Analyze(ctx context.Context, seq ast.Seq, source *data.Source, head *lakeparse.Commitish) (dag.Seq, error) {
	a := newAnalyzer(ctx, source, head)
	s := a.semSeq(seq)
	s = a.checkOutputs(true, s)
	if a.errors != nil {
		return nil, a.errors
	}
	return s, nil
}

// AnalyzeAddSource is the same as Analyze but it adds a default source if the
// DAG does not have one.
func AnalyzeAddSource(ctx context.Context, seq ast.Seq, source *data.Source, head *lakeparse.Commitish) (dag.Seq, error) {
	a := newAnalyzer(ctx, source, head)
	s := a.semSeq(seq)
	s = a.checkOutputs(true, s)
	if a.errors != nil {
		return nil, a.errors
	}
	if !HasSource(s) {
		if err := AddDefaultSource(ctx, &s, source, head); err != nil {
			return nil, err
		}
	}
	return s, nil
}

type analyzer struct {
	ctx     context.Context
	errors  parser.ErrorList
	head    *lakeparse.Commitish
	opStack []*ast.OpDecl
	outputs map[*dag.Output]ast.Node
	source  *data.Source
	scope   *Scope
	zctx    *super.Context
}

func newAnalyzer(ctx context.Context, source *data.Source, head *lakeparse.Commitish) *analyzer {
	return &analyzer{
		ctx:     ctx,
		head:    head,
		outputs: make(map[*dag.Output]ast.Node),
		source:  source,
		scope:   NewScope(nil),
		zctx:    super.NewContext(),
	}
}

func HasSource(seq dag.Seq) bool {
	switch op := seq[0].(type) {
	case *dag.FileScan, *dag.HTTPScan, *dag.PoolScan, *dag.LakeMetaScan, *dag.PoolMetaScan, *dag.CommitMetaScan, *dag.DeleteScan:
		return true
	case *dag.Fork:
		return HasSource(op.Paths[0])
	case *dag.Scope:
		return HasSource(op.Body)
	}
	return false
}

func AddDefaultSource(ctx context.Context, seq *dag.Seq, source *data.Source, head *lakeparse.Commitish) error {
	if HasSource(*seq) {
		return nil
	}
	// No from so add a source.
	if head == nil {
		seq.Prepend(&dag.DefaultScan{Kind: "DefaultScan"})
		return nil
	}
	// Verify pool exists for HEAD
	if _, err := source.PoolID(ctx, head.Pool); err != nil {
		return err
	}
	pool := &ast.Pool{
		Kind: "Pool",
		Spec: ast.PoolSpec{
			Pool: &ast.String{
				Kind: "String",
				Text: "HEAD",
			},
		},
	}
	ops := newAnalyzer(ctx, source, head).semPool(pool)
	seq.Prepend(ops[0])
	return nil
}

func StartsWithYield(seq dag.Seq) bool {
	switch op := seq[0].(type) {
	case *dag.Yield:
		return true
	case *dag.Scope:
		return StartsWithYield(op.Body)
	}
	return false
}

func (a *analyzer) enterScope() {
	a.scope = NewScope(a.scope)
}

func (a *analyzer) exitScope() {
	a.scope = a.scope.parent
}

type opDecl struct {
	ast   *ast.OpDecl
	scope *Scope // parent scope of op declaration.
	bad   bool
}

type opCycleError []*ast.OpDecl

func (e opCycleError) Error() string {
	b := "operator cycle found: "
	for i, op := range e {
		if i > 0 {
			b += " -> "
		}
		b += op.Name.Name
	}
	return b
}

func badExpr() dag.Expr {
	return &dag.BadExpr{Kind: "BadExpr"}
}

func badOp() dag.Op {
	return &dag.BadOp{Kind: "BadOp"}
}

func (a *analyzer) error(n ast.Node, err error) {
	a.errors.Append(err.Error(), n.Pos(), n.End())
}

func (a *analyzer) checkOutputs(isLeaf bool, seq dag.Seq) dag.Seq {
	if len(seq) == 0 {
		return seq
	}
	// - Report an error in any outputs are not located in the leaves.
	// - Add output operators to any leaves where they do not exist.
	lastN := len(seq) - 1
	for i, o := range seq {
		isLast := lastN == i
		switch o := o.(type) {
		case *dag.Output:
			if !isLast || !isLeaf {
				n, ok := a.outputs[o]
				if !ok {
					panic("system error: untracked user output")
				}
				a.error(n, errors.New("output operator must be at flowgraph leaf"))
			}
		case *dag.Scope:
			o.Body = a.checkOutputs(isLast && isLeaf, o.Body)
		case *dag.Scatter:
			for k := range o.Paths {
				o.Paths[k] = a.checkOutputs(isLast && isLeaf, o.Paths[k])
			}
		case *dag.Over:
			o.Body = a.checkOutputs(false, o.Body)
		case *dag.Fork:
			for k := range o.Paths {
				o.Paths[k] = a.checkOutputs(isLast && isLeaf, o.Paths[k])
			}
		case *dag.Switch:
			for k := range o.Cases {
				o.Cases[k].Path = a.checkOutputs(isLast && isLeaf, o.Cases[k].Path)
			}
		case *dag.Mirror:
			o.Main = a.checkOutputs(isLast && isLeaf, o.Main)
			o.Mirror = a.checkOutputs(isLast && isLeaf, o.Mirror)
		}
	}
	switch seq[lastN].(type) {
	case *dag.Scope, *dag.Output, *dag.Scatter, *dag.Fork, *dag.Switch, *dag.Mirror:
	default:
		if isLeaf {
			return append(seq, &dag.Output{Kind: "Output", Name: "main"})
		}
	}
	return seq
}
