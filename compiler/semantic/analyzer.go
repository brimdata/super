package semantic

import (
	"context"
	"errors"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/compiler/srcfiles"
	"github.com/brimdata/super/runtime/exec"
)

// Analyze performs a semantic analysis of the AST, translating it from AST
// to DAG form, resolving syntax ambiguities, and performing constant propagation.
// After semantic analysis, the DAG is ready for either optimization or compilation.
func Analyze(ctx context.Context, p *parser.AST, env *exec.Environment, extInput bool) (*dag.Main, error) {
	t := newTranslator(ctx, reporter{p.Files()}, env)
	astseq := p.Parsed()
	if extInput {
		astseq.Prepend(&ast.DefaultScan{Kind: "DefaultScan"})
	}
	seq := t.seq(astseq)
	if err := t.Error(); err != nil {
		return nil, err
	}
	if !HasSource(seq) {
		if t.env.IsAttached() {
			if len(seq) == 0 {
				return nil, errors.New("query text is missing")
			}
			seq.Prepend(&sem.NullScan{})
		} else {
			// This is a local query and there's no external input
			// (i.e., no command-line file args)
			seq.Prepend(&sem.NullScan{})
		}
	}
	newChecker(t).check(t.reporter, seq)
	if err := t.Error(); err != nil {
		return nil, err
	}
	main := newDagen(t.reporter).assemble(seq, t.resolver.funcs)
	return main, t.Error()
}

// Translate AST into semantic tree.  Resolve all bindings
// between symbols and their entities and flatten all scopes
// creating a global function table.  Convert SQL entities
// to dataflow.
type translator struct {
	reporter
	ctx      context.Context
	resolver *resolver
	opStack  []*ast.OpDecl
	cteStack []*ast.SQLCTE
	env      *exec.Environment
	scope    *Scope
	sctx     *super.Context
}

func newTranslator(ctx context.Context, r reporter, env *exec.Environment) *translator {
	t := &translator{
		reporter: r,
		ctx:      ctx,
		env:      env,
		scope:    NewScope(nil),
		sctx:     super.NewContext(),
	}
	t.resolver = newResolver(t)
	return t
}

func HasSource(seq sem.Seq) bool {
	if len(seq) == 0 {
		return false
	}
	switch op := seq[0].(type) {
	case *sem.FileScan, *sem.HTTPScan, *sem.PoolScan, *sem.DBMetaScan, *sem.PoolMetaScan, *sem.CommitMetaScan, *sem.DeleteScan, *sem.NullScan, *sem.DefaultScan:
		return true
	case *sem.ForkOp:
		for _, path := range op.Paths {
			if !HasSource(path) {
				return false
			}
		}
		return true
	}
	return false
}

func (t *translator) Error() error {
	return t.reporter.Error()
}

func (t *translator) enterScope() {
	t.scope = NewScope(t.scope)
}

func (t *translator) exitScope() {
	t.scope = t.scope.parent
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

func badExpr() sem.Expr {
	return &sem.BadExpr{}
}

func badOp() sem.Op {
	return &sem.BadOp{}
}

type reporter struct {
	*srcfiles.List
}

func (r reporter) error(n ast.Node, err error) {
	r.AddError(err.Error(), n.Pos(), n.End())
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
