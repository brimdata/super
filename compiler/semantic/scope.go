package semantic

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/kernel"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/zson"
)

type Scope struct {
	parent  *Scope
	nvar    int
	symbols map[string]*entry
	schema  schema
}

func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:  parent,
		symbols: make(map[string]*entry),
	}
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
	if err := s.DefineAs(name.Name, ref); err != nil {
		return err
	}
	s.nvar++
	return nil
}

func (s *Scope) DefineAs(name string, e any) error {
	if _, ok := s.symbols[name]; ok {
		return fmt.Errorf("symbol %q redefined", name)
	}
	s.symbols[name] = &entry{ref: e, order: len(s.symbols)}
	return nil
}

func (s *Scope) DefineSchema(schema schema) error {
	if err := s.DefineAs(schema.Name(), schema); err != nil {
		return err
	}
	s.nvar++
	return nil
}

func (s *Scope) DefineConst(zctx *super.Context, name *ast.ID, def dag.Expr) error {
	val, err := kernel.EvalAtCompileTime(zctx, def)
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
		Value: zson.FormatValue(val),
	}
	return s.DefineAs(name.Name, literal)
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
// and replace with dag path with dataflow encapsulation semantics.
// tab.col
// col.fld
// fld
// in the case of unqualified col ref, check that it is not ambiguous
// when there are multiple tables (i.e., from joins).
// an unqualified field reference is valid only in dynamic schemas.
// when resolving in a select-scope, for in match over out match.
// XXX hmm things that deal directly in path names are gonna need to be
// transformed, e.g., flatten and unflatten... we can start by simply disallowing
// these in SQL context

//XXX will need to handle lateral scopes so an inner select can
// move up the scope stack and reference out elements

func (s *Scope) resolve(path field.Path) (field.Path, error) {
	// If there's no schema, we're not in a SQL context so we just
	// return the path unmodified.  Otherwise, we apply SQL scoping
	// rules to transform the abstract path to the dataflow path
	// implied by the schema.
	if s.schema == nil || len(path) == 0 {
		return path, nil
	}
	if d, ok := s.schema.(*schemaDynamic); ok {
		return append([]string{d.name}, path...), nil
	}
	if len(path) == 1 {
		return resolveColumn(s.schema, path[0], nil)
	}
	if out, err := resolveTable(s.schema, path[0], path[1:]); out != nil || err != nil {
		return out, err
	}
	out, err := resolveColumn(s.schema, path[0], path[1:])
	if out == nil && err == nil {
		err = fmt.Errorf("%q: not a column or table", path[0])
	}
	return out, err
}

func resolveTable(schema schema, table string, path field.Path) (field.Path, error) {
	switch schema := schema.(type) {
	case *schemaDynamic:
		//XXX case insensitive
		if schema.name == table {
			return append([]string{schema.name}, path...), nil
		}
	case *schemaStatic:
		if schema.name == table {
			if len(path) == 0 {
				return []string{schema.name}, nil
			}
			out, err := resolveColumn(schema, path[0], path[1:])
			if err != nil {
				return nil, err
			}
			if out == nil {
				return nil, nil
			}
			return append([]string{schema.name}, out...), nil
		}
	case *schemaSelect:
		out, err := resolveTable(schema.in, table, path)
		if err != nil {
			return nil, err
		}
		if out != nil {
			return append([]string{"in"}, out...), nil
		}
		if schema.out != nil {
			out, err := resolveTable(schema.out, table, path)
			if err != nil {
				return nil, err
			}
			if out != nil {
				return append([]string{"out"}, out...), nil
			}
		}
	case *schemaJoin:
		out, err := resolveTable(schema.left, table, path)
		if err != nil {
			return nil, err
		}
		if out != nil {
			chk, err := resolveTable(schema.right, table, path)
			if err != nil {
				return nil, err
			}
			if chk != nil {
				return nil, fmt.Errorf("%q: ambiguous table reference", table)
			}
			return append([]string{"left"}, out...), nil
		}
		out, err = resolveTable(schema.right, table, path)
		if err != nil {
			return nil, err
		}
		if out != nil {
			return append([]string{"right"}, out...), nil
		}
	}
	return nil, nil
}

func resolveColumn(schema schema, col string, path field.Path) (field.Path, error) {
	switch schema := schema.(type) {
	case *schemaDynamic:
		return append([]string{schema.name, col}, path...), nil
	case *schemaStatic:
		//XXX column -> columns
		for _, c := range schema.column {
			if c == col {
				return append([]string{schema.name, col}, path...), nil
			}
		}
	case *schemaSelect:
		out, err := resolveColumn(schema.in, col, path)
		if err != nil {
			return nil, err
		}
		if out != nil {
			return append([]string{"in"}, out...), nil
		}
		if schema.out != nil {
			out, err := resolveColumn(schema.out, col, path)
			if err != nil {
				return nil, err
			}
			if out != nil {
				return append([]string{"out"}, out...), nil
			}
		}
	case *schemaJoin:
		out, err := resolveColumn(schema.left, col, path)
		if err != nil {
			return nil, err
		}
		if out != nil {
			chk, err := resolveColumn(schema.right, col, path)
			if err != nil {
				return nil, err
			}
			if chk != nil {
				return nil, fmt.Errorf("%q: ambiguous column reference", col)
			}
			return append([]string{"left"}, out...), nil
		}
		out, err = resolveColumn(schema.right, col, path)
		if err != nil {
			return nil, err
		}
		if out != nil {
			return append([]string{"right"}, out...), nil
		}
	}
	return nil, nil
}
