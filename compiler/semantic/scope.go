package semantic

import (
	"errors"
	"fmt"

	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/pkg/field"
)

type Scope struct {
	parent  *Scope
	symbols map[string]*entry
	ctes    map[string]*ast.SQLCTE
	schema  schema
}

func NewScope(parent *Scope) *Scope {
	return &Scope{parent: parent, symbols: make(map[string]*entry), ctes: make(map[string]*ast.SQLCTE)}
}

type entry struct {
	ref   any
	order int
}

type param struct{}

func (s *Scope) BindSymbol(name string, e any) error {
	if _, ok := s.symbols[name]; ok {
		return fmt.Errorf("symbol %q redefined", name)
	}
	s.symbols[name] = &entry{ref: e, order: len(s.symbols)}
	return nil
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

func (s *Scope) lookupQuery(t *translator, name string) sem.Seq {
	if entry := s.lookupEntry(name); entry != nil {
		if decl, ok := entry.ref.(*queryDecl); ok {
			return decl.resolve(t)
		}
	}
	return nil
}

func (s *Scope) lookupEntry(name string) *entry {
	for scope := s; scope != nil; scope = scope.parent {
		if entry, ok := scope.symbols[name]; ok {
			return entry
		}
	}
	return nil
}

func (s *Scope) lookupExpr(t *translator, name string) sem.Expr {
	if entry := s.lookupEntry(name); entry != nil {
		// function parameters hide exteral definitions as you don't
		// want the this.param ref to be overriden by a const etc.
		switch entry := entry.ref.(type) {
		case *sem.FuncDef, *ast.FuncNameExpr, param, *opDecl:
			return nil
		case *constDecl:
			return entry.resolve(t)
		}
		return entry.ref.(sem.Expr)
	}
	return nil
}

func (s *Scope) lookupFunc(name string) (string, error) {
	entry := s.lookupEntry(name)
	if entry == nil {
		return "", nil
	}
	switch ref := entry.ref.(type) {
	case *sem.FuncDef:
		return ref.Tag, nil
	case *sem.FuncRef:
		return ref.Tag, nil
	}
	return "", fmt.Errorf("%q is not bound to a function", name)
}

// resolve paths based on SQL semantics in order of precedence
// and replace with dag path with schemafied semantics.
// In the case of unqualified col ref, check that it is not ambiguous
// when there are multiple tables (i.e., from joins).
// An unqualified field reference is valid only in dynamic schemas.
func (s *Scope) resolve(n ast.Node, path field.Path) (sem.Expr, error) {
	// If there's no schema, we're not in a SQL context so we just
	// return the path unmodified.  Otherwise, we apply SQL scoping
	// rules to transform the abstract path to the dataflow path
	// implied by the schema.
	sch := s.schema
	if sch == nil {
		return sem.NewThis(n, path), nil
	}
	if len(path) == 0 {
		// XXX this should really treat this as a column in sql context but
		// but this will cause dynamic stuff to silently fail so I think we
		// should flag and maybe make it part of a strict mode (like bitwise |)
		if e := sch.this(n, nil); e != nil {
			return e, nil
		}
		return nil, errors.New("cannot reference 'this' in relational context; consider the 'values' operator") //XXX new error?
	}
	if len(path) == 1 {
		out, fatal, err := sch.resolveColumn(path[0])
		if fatal {
			return badExpr(), err
		}
		if err != nil {
			if e, err := sch.tableOnly(n, path[0], nil); err == nil {
				return e, nil
			}
			return badExpr(), err
		}
		if out != nil {
			return sem.NewThis(n, out), nil
		}
		return badExpr(), err
	}
	path, err := resolvePath(sch, path)
	if err != nil {
		return nil, err
	}
	return sem.NewThis(n, path), nil
}

func resolvePath(sch schema, path field.Path) (field.Path, error) {
	if len(path) <= 1 {
		panic("resolvePath")
	}
	table, tablePath, err := sch.resolveTable(path[0])
	if err != nil {
		return nil, err
	}
	if table != nil {
		columnPath, _, err := table.resolveColumn(path[1])
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
	out, _, err := sch.resolveColumn(path[0])
	if out == nil && err == nil {
		err = fmt.Errorf("%q: not a column or table", path[0])
	}
	return append(out, path[1:]...), err
}
