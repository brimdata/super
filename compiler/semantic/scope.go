package semantic

import (
	"errors"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/kernel"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/sup"
)

type Scope struct {
	parent  *Scope
	nvar    int
	symbols map[string]*entry
	ctes    map[string]*cte
	schema  schema
}

func NewScope(parent *Scope) *Scope {
	return &Scope{parent: parent, symbols: make(map[string]*entry), ctes: make(map[string]*cte)}
}

type entry struct {
	ref   any
	order int
}

type cte struct {
	body   dag.Seq
	schema schema
}

func (s *Scope) DefineVar(name *ast.ID) error {
	ref := &dag.Var{
		Kind: "Var",
		Name: name.Name,
		Slot: s.nvars(),
	}
	if err := s.DefineAs(name, ref); err != nil {
		return err
	}
	s.nvar++
	return nil
}

func (s *Scope) DefineAs(name *ast.ID, e any) error {
	if _, ok := s.symbols[name.Name]; ok {
		return fmt.Errorf("symbol %q redefined", name.Name)
	}
	s.symbols[name.Name] = &entry{ref: e, order: len(s.symbols)}
	return nil
}

func (s *Scope) DefineConst(sctx *super.Context, name *ast.ID, def dag.Expr) error {
	val, err := kernel.EvalAtCompileTime(sctx, def)
	if err != nil {
		return err
	}
	if val.IsError() {
		if val.IsMissing() {
			return fmt.Errorf("const %q: cannot have variable dependency", name.Name)
		} else {
			return fmt.Errorf("const %q: %q", name, string(val.Bytes()))
		}
	}
	literal := &dag.Literal{
		Kind:  "Literal",
		Value: sup.FormatValue(val),
	}
	return s.DefineAs(name, literal)
}

func (s *Scope) LookupExpr(name string) (dag.Expr, error) {
	if entry := s.lookupEntry(name); entry != nil {
		e, ok := entry.ref.(dag.Expr)
		if !ok {
			return nil, fmt.Errorf("symbol %q is not bound to an expression", name)
		}
		return e, nil
	}
	return nil, nil
}

func (s *Scope) lookupOp(name string) (*opDecl, error) {
	if entry := s.lookupEntry(name); entry != nil {
		d, ok := entry.ref.(*opDecl)
		if !ok {
			return nil, fmt.Errorf("symbol %q is not bound to an operator", name)
		}
		return d, nil
	}
	return nil, nil
}

func (s *Scope) lookupEntry(name string) *entry {
	for scope := s; scope != nil; scope = scope.parent {
		if entry, ok := scope.symbols[name]; ok {
			return entry
		}
	}
	return nil
}

func (s *Scope) nvars() int {
	var n int
	for scope := s; scope != nil; scope = scope.parent {
		n += scope.nvar
	}
	return n
}

// resolve paths based on SQL semantics in order of precedence
// and replace with dag path with schemafied semantics.
// In the case of unqualified col ref, check that it is not ambiguous
// when there are multiple tables (i.e., from joins).
// An unqualified field reference is valid only in dynamic schemas.
func (s *Scope) resolve(path field.Path) (dag.Expr, error) {
	// If there's no schema, we're not in a SQL context so we just
	// return the path unmodified.  Otherwise, we apply SQL scoping
	// rules to transform the abstract path to the dataflow path
	// implied by the schema.
	sch := s.schema
	if sch == nil {
		return &dag.This{Kind: "This", Path: path}, nil
	}
	if len(path) == 0 {
		// XXX this should really treat this as a column in sql context but
		// but this will cause dynamic stuff to silently fail so I think we
		// should flag and maybe make it part of a strict mode (like bitwise |)
		return nil, errors.New("cannot reference 'this' in relational context; consider the 'yield' operator")
	}
	path, err := resolvePath(sch, path)
	return &dag.This{Kind: "This", Path: path}, err
}

func resolvePath(sch schema, path field.Path) (field.Path, error) {
	if len(path) == 1 {
		return sch.resolveColumn(path[0])
	}
	table, tablePath, err := sch.resolveTable(path[0])
	if err != nil {
		return nil, err
	}
	if table != nil {
		columnPath, err := table.resolveColumn(path[1])
		if err != nil {
			return nil, fmt.Errorf("table %q: %w", path[0], err)
		}
		if columnPath != nil {
			out := append(tablePath, columnPath...)
			if len(path) > 2 {
				out = append(out, path[2:]...)
			}
			return out, nil
		}
	}
	out, err := sch.resolveColumn(path[0])
	if out == nil && err == nil {
		err = fmt.Errorf("%q: not a column or table", path[0])
	}
	return append(out, path[1:]...), err
}
