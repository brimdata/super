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
	"github.com/brimdata/super/sup"
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
	t.checker.pushErrs()
	seq, _ := t.seq(astseq, super.TypeNull)
	errs := t.checker.popErrs()
	errs.flushErrs(t.reporter)
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
	main := newDagen(t.reporter).assemble(seq, t.getTypes(), t.resolver.funcs)
	if env.Runtime == exec.RuntimeAuto && t.hasVectorizedInput {
		env.Runtime = exec.RuntimeVAM
	}
	return main, t.Error()
}

// Translate AST into semantic tree.  Resolve all bindings
// between symbols and their entities and flatten all scopes
// creating a global function table.  Convert SQL entities
// to dataflow.
type translator struct {
	reporter
	ctx                context.Context
	resolver           *resolver
	checker            *checker
	hasVectorizedInput bool
	opCnt              map[*ast.OpDecl]int
	opStack            []string
	cteStack           []*ast.SQLCTE
	env                *exec.Environment
	scope              *Scope
	sctx               *super.Context
	types              *sup.Analyzer
	defs               *super.Context
}

func newTranslator(ctx context.Context, r reporter, env *exec.Environment) *translator {
	defs := super.NewContext()
	t := &translator{
		reporter: r,
		ctx:      ctx,
		opCnt:    make(map[*ast.OpDecl]int),
		env:      env,
		scope:    NewScope(nil),
		sctx:     super.NewContext(),
		// We make a SUP analyzer to translate ast.Type entities in SuperSQL type decls
		// to the named type in the defs context.  This context will hold only the
		// types referred to by TypeRefs in the resulting sem tree.  This provides the
		// means to create dag.TypeRef references these types and serialize the defs tyepdefs
		// table into the DAG header.
		types: sup.NewAnalyzer(defs),
		defs:  defs,
	}
	t.checker = newChecker(t)
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

func (t *translator) getTypes() []byte {
	return t.defs.TypeDefs().Bytes()
}

// lookupTypeByID return the type from the typedefs table with ID
// in the translator sctx.
func (t *translator) lookupTypeByID(id int) super.Type {
	typ, err := t.defs.LookupType(id)
	if err != nil {
		panic(err)
	}
	typ, err = t.sctx.TranslateType(typ)
	if err != nil {
		panic(err)
	}
	return typ
}

type opDecl struct {
	ast   *ast.OpDecl
	scope *Scope // parent scope of op declaration.
	bad   bool
}

type opCycleError []string

func (e opCycleError) Error() string {
	var b strings.Builder
	b.WriteString("operator cycle found: ")
	for i, op := range e {
		if i > 0 {
			b.WriteString(" -> ")
		}
		b.WriteString(op)
	}
	return b.String()
}

var (
	badExpr  = &sem.BadExpr{}
	badOp    = &sem.BadOp{}
	badTable = &staticTable{}
	badType  = &super.TypeOfNull{}
)

type reporter struct {
	*srcfiles.List
}

func (r reporter) error(n ast.Node, err error) {
	r.AddError(err.Error(), n.Pos(), n.End())
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
