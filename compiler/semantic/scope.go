package semantic

import (
	"errors"
	"fmt"
	"strings"

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
	return &Scope{parent: parent, symbols: make(map[string]*entry)}
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
			return fmt.Errorf("const %q: cannot have variable dependency", name.Name)
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

// resolve paths based on SQL semantics in order of precedence
// and replace with dag path with schemafied semantics.
// In the case of unqualified col ref, check that it is not ambiguous
// when there are multiple tables (i.e., from joins).
// An unqualified field reference is valid only in dynamic schemas.
func (s *Scope) resolve(path field.Path) (field.Path, error) {
	// If there's no schema, we're not in a SQL context so we just
	// return the path unmodified.  Otherwise, we apply SQL scoping
	// rules to transform the abstract path to the dataflow path
	// implied by the schema.
	sch := s.schema
	if sch == nil {
		return path, nil
	}
	if len(path) == 0 {
		// XXX this should really treat this as a column in sql context but
		// but this will cause dynamic stuff to silently fail so I think we
		// should flag and maybe make it part of a strict mode (like bitwise |)
		return nil, errors.New("cannot reference 'this' in relational context; consider the 'yield' operator")
	}
	if len(path) == 1 {
		return sch.resolveColumn(path[0], nil)
	}
	if out, err := sch.resolveTable(path[0], path[1:]); out != nil || err != nil {
		return out, err
	}
	out, err := sch.resolveColumn(path[0], path[1:])
	if out == nil && err == nil {
		err = fmt.Errorf("%q: not a column or table", path[0])
	}
	return out, err
}

func (d *dynamicSchema) resolveTable(table string, path field.Path) (field.Path, error) {
	if strings.EqualFold(d.name, table) {
		return path, nil
	}
	return nil, nil
}

func (*anonSchema) resolveTable(table string, path field.Path) (field.Path, error) {
	return nil, nil
}

func (s *staticSchema) resolveTable(table string, path field.Path) (field.Path, error) {
	if strings.EqualFold(s.name, table) {
		if len(path) == 0 {
			return []string{}, nil
		}
		return s.resolveColumn(path[0], path[1:])
	}
	return nil, nil
}

func (s *selectSchema) resolveTable(table string, path field.Path) (field.Path, error) {
	if s.out != nil {
		target, err := s.out.resolveTable(table, path)
		if err != nil {
			return nil, err
		}
		if target != nil {
			return append([]string{"out"}, target...), nil
		}
	}
	target, err := s.in.resolveTable(table, path)
	if err != nil {
		return nil, err
	}
	if target != nil {
		return append([]string{"in"}, target...), nil
	}
	return nil, nil
}

func (j *joinSchema) resolveTable(table string, path field.Path) (field.Path, error) {
	out, err := j.left.resolveTable(table, path)
	if err != nil {
		return nil, err
	}
	if out != nil {
		chk, err := j.right.resolveTable(table, path)
		if err != nil {
			return nil, err
		}
		if chk != nil {
			return nil, fmt.Errorf("%q: ambiguous table reference", table)
		}
		return append([]string{"left"}, out...), nil
	}
	out, err = j.right.resolveTable(table, path)
	if err != nil {
		return nil, err
	}
	if out != nil {
		return append([]string{"right"}, out...), nil
	}
	return nil, nil
}

func (*dynamicSchema) resolveColumn(col string, path field.Path) (field.Path, error) {
	return append([]string{col}, path...), nil
}

func (s *staticSchema) resolveColumn(col string, path field.Path) (field.Path, error) {
	for _, c := range s.columns {
		if c == col {
			return append([]string{col}, path...), nil
		}
	}
	return nil, nil
}

func (a *anonSchema) resolveColumn(col string, path field.Path) (field.Path, error) {
	for _, c := range a.columns {
		if c == col {
			return append([]string{col}, path...), nil
		}
	}
	return nil, nil
}

func (s *selectSchema) resolveColumn(col string, path field.Path) (field.Path, error) {
	if s.out != nil {
		resolved, err := s.out.resolveColumn(col, path)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			return append([]string{"out"}, resolved...), nil
		}
	}
	resolved, err := s.in.resolveColumn(col, path)
	if err != nil {
		return nil, err
	}
	if resolved != nil {
		return append([]string{"in"}, resolved...), nil
	}
	return nil, nil
}

func (j *joinSchema) resolveColumn(col string, path field.Path) (field.Path, error) {
	out, err := j.left.resolveColumn(col, path)
	if err != nil {
		return nil, err
	}
	if out != nil {
		chk, err := j.right.resolveColumn(col, path)
		if err != nil {
			return nil, err
		}
		if chk != nil {
			return nil, fmt.Errorf("%q: ambiguous column reference", col)
		}
		return append([]string{"left"}, out...), nil
	}
	out, err = j.right.resolveColumn(col, path)
	if err != nil {
		return nil, err
	}
	if out != nil {
		return append([]string{"right"}, out...), nil
	}
	return nil, nil
}

func (d *dynamicSchema) deref(seq dag.Seq, name string) (dag.Seq, schema) {
	if name != "" {
		d = &dynamicSchema{name: name}
	}
	return seq, d
}

func (s *staticSchema) deref(seq dag.Seq, name string) (dag.Seq, schema) {
	if name != "" {
		s = &staticSchema{name: name, columns: s.columns}
	}
	return seq, s
}

func (a *anonSchema) deref(seq dag.Seq, name string) (dag.Seq, schema) {
	return seq, a
}

func (s *selectSchema) deref(seq dag.Seq, name string) (dag.Seq, schema) {
	if name == "" {
		// postgres and duckdb oddly do this
		name = "unamed_subquery"
	}
	var outSchema schema
	if anon, ok := s.out.(*anonSchema); ok {
		// Hide any enclosing schema hierarchy by just exporting the
		// select columns.
		outSchema = &staticSchema{name: name, columns: anon.columns}
	} else {
		// This is a select value.
		// XXX we should eventually have a way to propagate schema info here,
		// e.g., record expression with fixed columns as an anonSchema.
		outSchema = &dynamicSchema{}
	}
	return append(seq, &dag.Yield{
		Kind:  "Yield",
		Exprs: []dag.Expr{pathOf("out")},
	}), outSchema
}

func (j *joinSchema) deref(seq dag.Seq, name string) (dag.Seq, schema) {
	// spread left/right join legs into "this"
	e := &dag.RecordExpr{
		Kind: "RecordExpr",
		Elems: []dag.RecordElem{
			&dag.Spread{
				Kind: "Spread",
				Expr: &dag.This{Kind: "This", Path: []string{"left"}},
			},
			&dag.Spread{
				Kind: "Spread",
				Expr: &dag.This{Kind: "This", Path: []string{"right"}},
			},
		},
	}
	return append(seq, &dag.Yield{
		Kind:  "Yield",
		Exprs: []dag.Expr{e},
	}), &dynamicSchema{name: name}
}
