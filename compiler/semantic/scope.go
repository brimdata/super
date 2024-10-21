package semantic

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/ast/dag"
	"github.com/brimdata/super/compiler/kernel"
	"github.com/brimdata/super/zson"
)

type Scope struct {
	parent   *Scope
	children []*Scope
	nvar     int
	symbols  map[string]*entry
}

func NewScope(parent *Scope) *Scope {
	s := &Scope{parent: parent, symbols: make(map[string]*entry)}
	if parent != nil {
		parent.children = append(parent.children, s)
	}
	return s
}

type entry struct {
	ref   any
	order int
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

func (s *Scope) DefineConst(zctx *super.Context, name *ast.ID, def dag.Expr) error {
	val, err := kernel.EvalAtCompileTime(zctx, def)
	if err != nil {
		return err
	}
	if val.IsError() {
		if val.IsMissing() {
			return fmt.Errorf("const %q: cannot have variable dependency", name)
		} else {
			return fmt.Errorf("const %q: %q", name, string(val.Bytes()))
		}
	}
	literal := &dag.Literal{
		Kind:  "Literal",
		Value: zson.FormatValue(val),
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
