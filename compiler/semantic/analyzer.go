package semantic

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/compiler/srcfiles"
	"github.com/brimdata/super/runtime/exec"
)

// Analyze performs a semantic analysis of the AST, translating it from AST
// to DAG form, resolving syntax ambiguities, and performing constant propagation.
// After semantic analysis, the DAG is ready for either optimization or compilation.
func Analyze(ctx context.Context, p *parser.AST, env *exec.Environment, extInput bool) (*dag.Main, error) {
	files := p.Files()
	a := newAnalyzer(ctx, files, env)
	astseq := p.Parsed()
	if extInput {
		astseq.Prepend(&ast.DefaultScan{Kind: "DefaultScan"})
	}
	seq := a.semSeq(astseq)
	if !HasSource(seq) {
		if a.env.IsAttached() {
			if len(seq) == 0 {
				return nil, errors.New("query text is missing")
			}
			seq.Prepend(&dag.NullScan{Kind: "NullScan"})
		} else {
			// This is a local query and there's no external input
			// (i.e., no command-line file args)
			seq.Prepend(&dag.NullScan{Kind: "NullScan"})
		}
	}
	seq = a.checkOutputs(true, seq)
	r := newResolver(a)
	seq, funcs := r.resolve(seq)
	// Sort function entries so they are consistently ordered by integer tag strings.
	slices.SortFunc(funcs, func(a, b *dag.FuncDef) int {
		return strings.Compare(a.Tag, b.Tag)
	})
	return &dag.Main{Funcs: funcs, Body: seq}, files.Error()
}

type analyzer struct {
	ctx         context.Context
	files       *srcfiles.List
	opStack     []*ast.OpDecl
	cteStack    []*cte
	outputs     map[*dag.Output]ast.Node
	env         *exec.Environment
	scope       *Scope
	sctx        *super.Context
	funcsByTag  map[string]*dag.FuncDef
	funcsByDecl map[*ast.Decl]*dag.FuncDef
	locs        map[dag.Expr]ast.Loc
}

func newAnalyzer(ctx context.Context, files *srcfiles.List, env *exec.Environment) *analyzer {
	return &analyzer{
		ctx:         ctx,
		files:       files,
		outputs:     make(map[*dag.Output]ast.Node),
		env:         env,
		scope:       NewScope(nil),
		sctx:        super.NewContext(),
		funcsByTag:  make(map[string]*dag.FuncDef),
		funcsByDecl: make(map[*ast.Decl]*dag.FuncDef),
		locs:        make(map[dag.Expr]ast.Loc),
	}
}

func HasSource(seq dag.Seq) bool {
	if len(seq) == 0 {
		return false
	}
	switch op := seq[0].(type) {
	case *dag.FileScan, *dag.HTTPScan, *dag.PoolScan, *dag.DBMetaScan, *dag.PoolMetaScan, *dag.CommitMetaScan, *dag.DeleteScan, *dag.NullScan, *dag.DefaultScan:
		return true
	case *dag.Fork:
		for _, path := range op.Paths {
			if !HasSource(path) {
				return false
			}
		}
		return true
	}
	return false
}

func (a *analyzer) enterScope() {
	a.scope = NewScope(a.scope)
}

func (a *analyzer) exitScope() {
	a.scope = a.scope.parent
}

func (a *analyzer) newFunc(name string, params []string, e dag.Expr) string {
	tag := strconv.Itoa(len(a.funcsByTag))
	a.funcsByTag[tag] = &dag.FuncDef{
		Kind:   "FuncDef",
		Tag:    tag,
		Name:   name,
		Params: params,
		Expr:   e,
	}
	return tag
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
	a.files.AddError(err.Error(), n.Pos(), n.End())
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
		case *dag.Scatter:
			for k := range o.Paths {
				o.Paths[k] = a.checkOutputs(isLast && isLeaf, o.Paths[k])
			}
		case *dag.Unnest:
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
	case *dag.Output, *dag.Scatter, *dag.Fork, *dag.Switch, *dag.Mirror:
	default:
		if isLeaf {
			return append(seq, &dag.Output{Kind: "Output", Name: "main"})
		}
	}
	return seq
}
