package semantic

import (
	"errors"
	"fmt"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/compiler/ast"
	"github.com/brimdata/zed/compiler/ast/dag"
	astzed "github.com/brimdata/zed/compiler/ast/zed"
	"github.com/brimdata/zed/compiler/kernel"
	"github.com/brimdata/zed/pkg/reglob"
	"github.com/brimdata/zed/runtime/expr"
	"github.com/brimdata/zed/runtime/expr/agg"
	"github.com/brimdata/zed/zson"
)

func (a *analyzer) semExpr(e ast.Expr) (dag.Expr, error) {
	switch e := e.(type) {
	case nil:
		return nil, errors.New("semantic analysis: illegal null value encountered in AST")
	case *ast.Regexp:
		return &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: e.Pattern,
			Expr:    pathOf("this"),
		}, nil
	case *ast.Glob:
		return &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: reglob.Reglob(e.Pattern),
			Expr:    pathOf("this"),
		}, nil
	case *ast.Grep:
		return a.semGrep(e)
	case *astzed.Primitive:
		val, err := zson.ParsePrimitive(e.Type, e.Text)
		if err != nil {
			return nil, err
		}
		return &dag.Literal{
			Kind:  "Literal",
			Value: zson.MustFormatValue(val),
		}, nil
	case *ast.ID:
		return a.semID(e), nil
	case *ast.Term:
		var val string
		switch t := e.Value.(type) {
		case *astzed.Primitive:
			v, err := zson.ParsePrimitive(t.Type, t.Text)
			if err != nil {
				return nil, err
			}
			val = zson.MustFormatValue(v)
		case *astzed.TypeValue:
			tv, err := a.semType(t.Value)
			if err != nil {
				return nil, err
			}
			val = "<" + tv + ">"
		default:
			return nil, fmt.Errorf("unexpected term value: %s", e.Kind)
		}
		return &dag.Search{
			Kind:  "Search",
			Text:  e.Text,
			Value: val,
			Expr:  pathOf("this"),
		}, nil
	case *ast.UnaryExpr:
		expr, err := a.semExpr(e.Operand)
		if err != nil {
			return nil, err
		}
		return &dag.UnaryExpr{
			Kind:    "UnaryExpr",
			Op:      e.Op,
			Operand: expr,
		}, nil
	case *ast.BinaryExpr:
		return a.semBinary(e)
	case *ast.Conditional:
		cond, err := a.semExpr(e.Cond)
		if err != nil {
			return nil, err
		}
		thenExpr, err := a.semExpr(e.Then)
		if err != nil {
			return nil, err
		}
		elseExpr, err := a.semExpr(e.Else)
		if err != nil {
			return nil, err
		}
		return &dag.Conditional{
			Kind: "Conditional",
			Cond: cond,
			Then: thenExpr,
			Else: elseExpr,
		}, nil
	case *ast.Call:
		return a.semCall(e)
	case *ast.Cast:
		expr, err := a.semExpr(e.Expr)
		if err != nil {
			return nil, err
		}
		typ, err := a.semExpr(e.Type)
		if err != nil {
			return nil, err
		}
		return &dag.Call{
			Kind: "Call",
			Name: "cast",
			Args: []dag.Expr{expr, typ},
		}, nil
	case *astzed.TypeValue:
		typ, err := a.semType(e.Value)
		if err != nil {
			// If this is a type name, then we check to see if it's in the
			// context because it has been defined locally.  If not, then
			// the type needs to come from the data, in which case we replace
			// the literal reference with a typename() call.
			// Note that we just check the top value here but there can be
			// nested dynamic type references inside a complex type; this
			// is not yet supported and will fail here with a compile-time error
			// complaining about the type not existing.
			// XXX See issue #3413
			if e := semDynamicType(e.Value); e != nil {
				return e, nil
			}
			return nil, err
		}
		return &dag.Literal{
			Kind:  "Literal",
			Value: "<" + typ + ">",
		}, nil
	case *ast.Agg:
		expr, err := a.semExprNullable(e.Expr)
		if err != nil {
			return nil, err
		}
		if expr == nil && e.Name != "count" {
			return nil, fmt.Errorf("aggregator '%s' requires argument", e.Name)
		}
		where, err := a.semExprNullable(e.Where)
		if err != nil {
			return nil, err
		}
		return &dag.Agg{
			Kind:  "Agg",
			Name:  e.Name,
			Expr:  expr,
			Where: where,
		}, nil
	case *ast.RecordExpr:
		var out []dag.RecordElem
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *ast.Field:
				e, err := a.semExpr(elem.Value)
				if err != nil {
					return nil, err
				}
				out = append(out, &dag.Field{
					Kind:  "Field",
					Name:  elem.Name,
					Value: e,
				})
			case *ast.ID:
				out = append(out, &dag.Field{
					Kind:  "Field",
					Name:  elem.Name,
					Value: a.semID(elem),
				})
			case *ast.Spread:
				e, err := a.semExpr(elem.Expr)
				if err != nil {
					return nil, err
				}
				out = append(out, &dag.Spread{
					Kind: "Spread",
					Expr: e,
				})
			}
		}
		return &dag.RecordExpr{
			Kind:  "RecordExpr",
			Elems: out,
		}, nil
	case *ast.ArrayExpr:
		elems, err := a.semVectorElems(e.Elems)
		if err != nil {
			return nil, err
		}
		return &dag.ArrayExpr{
			Kind:  "ArrayExpr",
			Elems: elems,
		}, nil
	case *ast.SetExpr:
		elems, err := a.semVectorElems(e.Elems)
		if err != nil {
			return nil, err
		}
		return &dag.SetExpr{
			Kind:  "SetExpr",
			Elems: elems,
		}, nil
	case *ast.MapExpr:
		var entries []dag.Entry
		for _, entry := range e.Entries {
			key, err := a.semExpr(entry.Key)
			if err != nil {
				return nil, err
			}
			val, err := a.semExpr(entry.Value)
			if err != nil {
				return nil, err
			}
			entries = append(entries, dag.Entry{Key: key, Value: val})
		}
		return &dag.MapExpr{
			Kind:    "MapExpr",
			Entries: entries,
		}, nil
	case *ast.OverExpr:
		exprs, err := a.semExprs(e.Exprs)
		if err != nil {
			return nil, err
		}
		if e.Body == nil {
			return nil, errors.New("over expression missing lateral scope")
		}
		a.scope.Enter()
		defer a.scope.Exit()
		locals, err := a.semVars(e.Locals)
		if err != nil {
			return nil, err
		}
		body, err := a.semSeq(e.Body)
		if err != nil {
			return nil, err
		}
		return &dag.OverExpr{
			Kind:  "OverExpr",
			Defs:  locals,
			Exprs: exprs,
			Body:  body,
		}, nil
	}
	return nil, fmt.Errorf("invalid expression type %T", e)
}

func (a *analyzer) semID(id *ast.ID) dag.Expr {
	// We use static scoping here to see if an identifier is
	// a "var" reference to the name or a field access
	// and transform the AST node appropriately.  The resulting DAG
	// doesn't have Identifiers as they are resolved here
	// one way or the other.
	if ref := a.scope.Lookup(id.Name); ref != nil {
		return ref
	}
	return pathOf(id.Name)
}

func semDynamicType(tv astzed.Type) *dag.Call {
	if typeName, ok := tv.(*astzed.TypeName); ok {
		return dynamicTypeName(typeName.Name)
	}
	return nil
}

func dynamicTypeName(name string) *dag.Call {
	return &dag.Call{
		Kind: "Call",
		Name: "typename",
		Args: []dag.Expr{
			// ZSON string literal of type name
			&dag.Literal{
				Kind:  "Literal",
				Value: `"` + name + `"`,
			},
		},
	}
}

func (a *analyzer) semGrep(grep *ast.Grep) (dag.Expr, error) {
	e, err := a.semExpr(grep.Expr)
	if err != nil {
		return nil, err
	}
	switch pattern := grep.Pattern.(type) {
	case *ast.String:
		return &dag.Search{
			Kind:  "Search",
			Text:  pattern.Text,
			Value: zson.QuotedString([]byte(pattern.Text)),
			Expr:  e,
		}, nil
	case *ast.Regexp:
		return &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: pattern.Pattern,
			Expr:    e,
		}, nil
	case *ast.Glob:
		return &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: reglob.Reglob(pattern.Pattern),
			Expr:    e,
		}, nil
	default:
		return nil, fmt.Errorf("semantic analyzer: unknown grep pattern %T", pattern)
	}
}

func (a *analyzer) semRegexp(b *ast.BinaryExpr) (dag.Expr, error) {
	if b.Op != "~" {
		return nil, nil
	}
	re, ok := b.RHS.(*ast.Regexp)
	if !ok {
		return nil, errors.New(`right-hand side of ~ expression must be a regular expression`)
	}
	if _, err := expr.CompileRegexp(re.Pattern); err != nil {
		return nil, err
	}
	e, err := a.semExpr(b.LHS)
	if err != nil {
		return nil, err
	}
	return &dag.RegexpMatch{
		Kind:    "RegexpMatch",
		Pattern: re.Pattern,
		Expr:    e,
	}, nil
}

func (a *analyzer) semBinary(e *ast.BinaryExpr) (dag.Expr, error) {
	if e, err := a.semRegexp(e); e != nil || err != nil {
		return e, err
	}
	op := e.Op
	if op == "." {
		lhs, err := a.semExpr(e.LHS)
		if err != nil {
			return nil, err
		}
		id, ok := e.RHS.(*ast.ID)
		if !ok {
			return nil, errors.New("RHS of dot operator is not an identifier")
		}
		if lhs, ok := lhs.(*dag.This); ok {
			lhs.Path = append(lhs.Path, id.Name)
			return lhs, nil
		}
		return &dag.Dot{
			Kind: "Dot",
			LHS:  lhs,
			RHS:  id.Name,
		}, nil
	}
	if slice, ok := e.RHS.(*ast.BinaryExpr); ok && slice.Op == ":" {
		if op != "[" {
			return nil, errors.New("slice outside of index operator")
		}
		ref, err := a.semExpr(e.LHS)
		if err != nil {
			return nil, err
		}
		slice, err := a.semSlice(slice)
		if err != nil {
			return nil, err
		}
		return &dag.BinaryExpr{
			Kind: "BinaryExpr",
			Op:   "[",
			LHS:  ref,
			RHS:  slice,
		}, nil
	}
	lhs, err := a.semExpr(e.LHS)
	if err != nil {
		return nil, err
	}
	rhs, err := a.semExpr(e.RHS)
	if err != nil {
		return nil, err
	}
	// If we index this with a string constant, then just
	// extend the path.
	if op == "[" {
		if path := a.isIndexOfThis(lhs, rhs); path != nil {
			return path, nil
		}
	}
	return &dag.BinaryExpr{
		Kind: "BinaryExpr",
		Op:   e.Op,
		LHS:  lhs,
		RHS:  rhs,
	}, nil
}

func (a *analyzer) isIndexOfThis(lhs, rhs dag.Expr) *dag.This {
	if this, ok := lhs.(*dag.This); ok {
		if s, ok := isStringConst(a.zctx, rhs); ok {
			this.Path = append(this.Path, s)
			return this
		}
	}
	return nil
}

func isStringConst(zctx *zed.Context, e dag.Expr) (field string, ok bool) {
	val, err := kernel.EvalAtCompileTime(zctx, e)
	if err == nil && !val.IsError() && zed.TypeUnder(val.Type) == zed.TypeString {
		return string(val.Bytes()), true
	}
	return "", false
}

func (a *analyzer) semSlice(slice *ast.BinaryExpr) (*dag.BinaryExpr, error) {
	sliceFrom, err := a.semExprNullable(slice.LHS)
	if err != nil {
		return nil, err
	}
	sliceTo, err := a.semExprNullable(slice.RHS)
	if err != nil {
		return nil, err
	}
	return &dag.BinaryExpr{
		Kind: "BinaryExpr",
		Op:   ":",
		LHS:  sliceFrom,
		RHS:  sliceTo,
	}, nil
}

func (a *analyzer) semExprNullable(e ast.Expr) (dag.Expr, error) {
	if e == nil {
		return nil, nil
	}
	return a.semExpr(e)
}

func (a *analyzer) semCall(call *ast.Call) (dag.Expr, error) {
	if e, err := a.maybeConvertAgg(call); e != nil || err != nil {
		return e, err
	}
	if call.Where != nil {
		return nil, fmt.Errorf("'where' clause on non-aggregation function: %s", call.Name)
	}
	exprs, err := a.semExprs(call.Args)
	if err != nil {
		return nil, fmt.Errorf("%s: bad argument: %w", call.Name, err)
	}
	// Call could be to a user defined func. Check if we have a matching func in
	// scope.
	if e := a.scope.Lookup(call.Name); e != nil {
		f, ok := e.(*dag.Func)
		if !ok {
			return nil, fmt.Errorf("%s(): definition is not a function type: %T", call.Name, e)
		}
		if len(f.Params) != len(call.Args) {
			return nil, fmt.Errorf("%s(): expects %d argument(s)", call.Name, len(f.Params))
		}
	}
	return &dag.Call{
		Kind: "Call",
		Name: call.Name,
		Args: exprs,
	}, nil
}

func (a *analyzer) semExprs(in []ast.Expr) ([]dag.Expr, error) {
	exprs := make([]dag.Expr, 0, len(in))
	for _, e := range in {
		expr, err := a.semExpr(e)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
	}
	return exprs, nil
}

func (a *analyzer) semAssignments(assignments []ast.Assignment, summarize bool) ([]dag.Assignment, error) {
	out := make([]dag.Assignment, 0, len(assignments))
	for _, e := range assignments {
		a, err := a.semAssignment(e, summarize)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (a *analyzer) semAssignment(assign ast.Assignment, summarize bool) (dag.Assignment, error) {
	rhs, err := a.semExpr(assign.RHS)
	if err != nil {
		return dag.Assignment{}, fmt.Errorf("rhs of assignment expression: %w", err)
	}
	if _, ok := rhs.(*dag.Agg); ok {
		summarize = true
	}
	var lhs dag.Expr
	if assign.LHS != nil {
		lhs, err = a.semExpr(assign.LHS)
		if err != nil {
			return dag.Assignment{}, fmt.Errorf("lhs of assigment expression: %w", err)
		}
	} else if call, ok := assign.RHS.(*ast.Call); ok {
		path := []string{call.Name}
		switch call.Name {
		case "every":
			// If LHS is nil and the call is every() make the LHS field ts since
			// field ts assumed with every.
			path = []string{"ts"}
		case "quiet":
			if len(call.Args) > 0 {
				if p, ok := rhs.(*dag.Call).Args[0].(*dag.This); ok {
					path = p.Path
				}
			}
		}
		lhs = &dag.This{Kind: "This", Path: path}
	} else if agg, ok := assign.RHS.(*ast.Agg); ok {
		lhs = &dag.This{Kind: "This", Path: []string{agg.Name}}
	} else if v, ok := rhs.(*dag.Var); ok {
		lhs = &dag.This{Kind: "This", Path: []string{v.Name}}
	} else {
		lhs, err = a.semExpr(assign.RHS)
		if err != nil {
			return dag.Assignment{}, errors.New("assignment name could not be inferred from rhs expression")
		}
	}
	if summarize {
		// Summarize always outputs its results as new records of "this"
		// so if we have an "as" that overrides "this", we just
		// convert it back to a local this.
		if dot, ok := lhs.(*dag.Dot); ok {
			if v, ok := dot.LHS.(*dag.Var); ok && v.Name == "this" {
				lhs = &dag.This{Kind: "This", Path: []string{dot.RHS}}
			}
		}
	}
	// Make sure we have a valid lval for lhs.
	this, ok := lhs.(*dag.This)
	if !ok {
		return dag.Assignment{}, errors.New("illegal left-hand side of assignment")
	}
	if len(this.Path) == 0 {
		return dag.Assignment{}, errors.New("cannot assign to 'this'")
	}
	return dag.Assignment{Kind: "Assignment", LHS: lhs, RHS: rhs}, nil
}

func (a *analyzer) semFields(exprs []ast.Expr) ([]dag.Expr, error) {
	fields := make([]dag.Expr, 0, len(exprs))
	for _, e := range exprs {
		f, err := a.semField(e)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	return fields, nil
}

// semField analyzes the expression f and makes sure that it's
// a field reference returning an error if not.
func (a *analyzer) semField(f ast.Expr) (*dag.This, error) {
	e, err := a.semExpr(f)
	if err != nil {
		return nil, errors.New("invalid expression used as a field")
	}
	field, ok := e.(*dag.This)
	if !ok {
		return nil, errors.New("invalid expression used as a field")
	}
	if len(field.Path) == 0 {
		return nil, errors.New("cannot use 'this' as a field reference")
	}
	return field, nil
}

func (a *analyzer) maybeConvertAgg(call *ast.Call) (dag.Expr, error) {
	if _, err := agg.NewPattern(call.Name, true); err != nil {
		return nil, nil
	}
	var e dag.Expr
	if len(call.Args) > 1 {
		if call.Name == "min" || call.Name == "max" {
			// min and max are special cases as they are also functions. If the
			// number of args is greater than 1 they're probably a function so do not
			// return an error.
			return nil, nil
		}
		return nil, fmt.Errorf("%s: wrong number of arguments", call.Name)
	}
	if len(call.Args) == 1 {
		var err error
		e, err = a.semExpr(call.Args[0])
		if err != nil {
			return nil, err
		}
	}
	where, err := a.semExprNullable(call.Where)
	if err != nil {
		return nil, err
	}
	return &dag.Agg{
		Kind:  "Agg",
		Name:  call.Name,
		Expr:  e,
		Where: where,
	}, nil
}

func DotExprToFieldPath(e ast.Expr) *dag.This {
	switch e := e.(type) {
	case *ast.BinaryExpr:
		if e.Op == "." {
			lhs := DotExprToFieldPath(e.LHS)
			if lhs == nil {
				return nil
			}
			id, ok := e.RHS.(*ast.ID)
			if !ok {
				return nil
			}
			lhs.Path = append(lhs.Path, id.Name)
			return lhs
		}
		if e.Op == "[" {
			lhs := DotExprToFieldPath(e.LHS)
			if lhs == nil {
				return nil
			}
			id, ok := e.RHS.(*astzed.Primitive)
			if !ok || id.Type != "string" {
				return nil
			}
			lhs.Path = append(lhs.Path, id.Text)
			return lhs
		}
	case *ast.ID:
		return pathOf(e.Name)
	}
	// This includes a null Expr, which can happen if the AST is missing
	// a field or sets it to null.
	return nil
}

func pathOf(name string) *dag.This {
	var path []string
	if name != "this" {
		path = []string{name}
	}
	return &dag.This{Kind: "This", Path: path}
}

func (a *analyzer) semType(typ astzed.Type) (string, error) {
	ztype, err := zson.TranslateType(a.zctx, typ)
	if err != nil {
		return "", err
	}
	return zson.FormatType(ztype), nil
}

func (a *analyzer) semVectorElems(elems []ast.VectorElem) ([]dag.VectorElem, error) {
	var out []dag.VectorElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *ast.Spread:
			e, err := a.semExpr(elem.Expr)
			if err != nil {
				return nil, err
			}
			out = append(out, &dag.Spread{Kind: "Spread", Expr: e})
		case *ast.VectorValue:
			e, err := a.semExpr(elem.Expr)
			if err != nil {
				return nil, err
			}
			out = append(out, &dag.VectorValue{Kind: "VectorValue", Expr: e})
		}
	}
	return out, nil
}
