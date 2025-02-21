package semantic

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/kernel"
	"github.com/brimdata/super/pkg/reglob"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/zson"
	"github.com/shellyln/go-sql-like-expr/likeexpr"
)

func (a *analyzer) semExpr(e ast.Expr) dag.Expr {
	switch e := e.(type) {
	case nil:
		panic("semantic analysis: illegal null value encountered in AST")
	case *ast.Regexp:
		return &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: e.Pattern,
			Expr:    pathOf("this"),
		}
	case *ast.Glob:
		return &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: reglob.Reglob(e.Pattern),
			Expr:    pathOf("this"),
		}
	case *ast.Grep:
		return a.semGrep(e)
	case *ast.Primitive:
		val, err := zson.ParsePrimitive(e.Type, e.Text)
		if err != nil {
			a.error(e, err)
			return badExpr()
		}
		return &dag.Literal{
			Kind:  "Literal",
			Value: zson.FormatValue(val),
		}
	case *ast.ID:
		id := a.semID(e)
		if a.scope.schema != nil {
			if this, ok := id.(*dag.This); ok {
				path, err := a.scope.resolve(this.Path)
				if err != nil {
					a.error(e, err)
					return badExpr()
				}
				return &dag.This{Kind: "This", Path: path}
			}
		}
		return id
	case *ast.Term:
		var val string
		switch t := e.Value.(type) {
		case *ast.Primitive:
			v, err := zson.ParsePrimitive(t.Type, t.Text)
			if err != nil {
				a.error(e, err)
				return badExpr()
			}
			val = zson.FormatValue(v)
		case *ast.TypeValue:
			tv, err := a.semType(t.Value)
			if err != nil {
				a.error(e, err)
				return badExpr()
			}
			val = "<" + tv + ">"
		default:
			panic(fmt.Errorf("unexpected term value: %s", e.Kind))
		}
		return &dag.Search{
			Kind:  "Search",
			Text:  e.Text,
			Value: val,
			Expr:  pathOf("this"),
		}
	case *ast.UnaryExpr:
		operand := a.semExpr(e.Operand)
		if e.Op == "+" {
			return operand
		}
		return &dag.UnaryExpr{
			Kind:    "UnaryExpr",
			Op:      e.Op,
			Operand: operand,
		}
	case *ast.BinaryExpr:
		return a.semBinary(e)
	case *ast.Between:
		val := a.semExpr(e.Expr)
		lower := a.semExpr(e.Lower)
		upper := a.semExpr(e.Upper)
		return &dag.BinaryExpr{
			Kind: "BinaryExpr",
			Op:   "and",
			LHS: &dag.BinaryExpr{
				Kind: "BinaryExpr",
				Op:   ">=",
				LHS:  val,
				RHS:  lower,
			},
			RHS: &dag.BinaryExpr{
				Kind: "BinaryExpr",
				Op:   "<=",
				LHS:  val,
				RHS:  upper,
			},
		}
	case *ast.Conditional:
		cond := a.semExpr(e.Cond)
		thenExpr := a.semExpr(e.Then)
		var elseExpr dag.Expr
		if e.Else != nil {
			elseExpr = a.semExpr(e.Else)
		} else {
			elseExpr = &dag.Literal{
				Kind:  "Literal",
				Value: `error("missing")`,
			}
		}
		return &dag.Conditional{
			Kind: "Conditional",
			Cond: cond,
			Then: thenExpr,
			Else: elseExpr,
		}
	case *ast.Call:
		return a.semCall(e)
	case *ast.Cast:
		expr := a.semExpr(e.Expr)
		typ := a.semExpr(e.Type)
		return &dag.Call{
			Kind: "Call",
			Name: "cast",
			Args: []dag.Expr{expr, typ},
		}
	case *ast.IndexExpr:
		expr := a.semExpr(e.Expr)
		index := a.semExpr(e.Index)
		// If expr is a path and index is a string, then just extend the path.
		if path := a.isIndexOfThis(expr, index); path != nil {
			return path
		}
		return &dag.IndexExpr{
			Kind:  "IndexExpr",
			Expr:  expr,
			Index: index,
		}
	case *ast.IsNullExpr:
		expr := a.semExpr(e.Expr)
		var out dag.Expr = &dag.IsNullExpr{Kind: "IsNullExpr", Expr: expr}
		if e.Not {
			out = &dag.UnaryExpr{
				Kind:    "UnaryExpr",
				Op:      "!",
				Operand: out,
			}
		}
		return out
	case *ast.SliceExpr:
		expr := a.semExpr(e.Expr)
		// XXX Literal indices should be type checked as int.
		from := a.semExprNullable(e.From)
		to := a.semExprNullable(e.To)
		return &dag.SliceExpr{
			Kind: "SliceExpr",
			Expr: expr,
			From: from,
			To:   to,
		}
	case *ast.TypeValue:
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
				return e
			}
			a.error(e, err)
			return badExpr()
		}
		return &dag.Literal{
			Kind:  "Literal",
			Value: "<" + typ + ">",
		}
	case *ast.Agg:
		expr := a.semExprNullable(e.Expr)
		nameLower := strings.ToLower(e.Name)
		if expr == nil && nameLower != "count" {
			a.error(e, fmt.Errorf("aggregator '%s' requires argument", e.Name))
			return badExpr()
		}
		where := a.semExprNullable(e.Where)
		return &dag.Agg{
			Kind:  "Agg",
			Name:  nameLower,
			Expr:  expr,
			Where: where,
		}
	case *ast.RecordExpr:
		fields := map[string]struct{}{}
		var out []dag.RecordElem
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *ast.FieldExpr:
				if _, ok := fields[elem.Name.Text]; ok {
					a.error(elem, fmt.Errorf("record expression: %w", &super.DuplicateFieldError{Name: elem.Name.Text}))
					continue
				}
				fields[elem.Name.Text] = struct{}{}
				e := a.semExpr(elem.Value)
				out = append(out, &dag.Field{
					Kind:  "Field",
					Name:  elem.Name.Text,
					Value: e,
				})
			case *ast.ID:
				if _, ok := fields[elem.Name]; ok {
					a.error(elem, fmt.Errorf("record expression: %w", &super.DuplicateFieldError{Name: elem.Name}))
					continue
				}
				fields[elem.Name] = struct{}{}
				// Call semExpr even though we know this is an ID so
				// SQL-context scope mappings are carried out.
				v := a.semExpr(elem)
				out = append(out, &dag.Field{
					Kind:  "Field",
					Name:  elem.Name,
					Value: v,
				})
			case *ast.Spread:
				e := a.semExpr(elem.Expr)
				out = append(out, &dag.Spread{
					Kind: "Spread",
					Expr: e,
				})
			}
		}
		return &dag.RecordExpr{
			Kind:  "RecordExpr",
			Elems: out,
		}
	case *ast.ArrayExpr:
		elems := a.semVectorElems(e.Elems)
		return &dag.ArrayExpr{
			Kind:  "ArrayExpr",
			Elems: elems,
		}
	case *ast.SetExpr:
		elems := a.semVectorElems(e.Elems)
		return &dag.SetExpr{
			Kind:  "SetExpr",
			Elems: elems,
		}
	case *ast.MapExpr:
		var entries []dag.Entry
		for _, entry := range e.Entries {
			key := a.semExpr(entry.Key)
			val := a.semExpr(entry.Value)
			entries = append(entries, dag.Entry{Key: key, Value: val})
		}
		return &dag.MapExpr{
			Kind:    "MapExpr",
			Entries: entries,
		}
	case *ast.TupleExpr:
		elems := make([]dag.RecordElem, 0, len(e.Elems))
		for colno, elem := range e.Elems {
			e := a.semExpr(elem)
			elems = append(elems, &dag.Field{
				Kind:  "Field",
				Name:  fmt.Sprintf("c%d", colno),
				Value: e,
			})
		}
		return &dag.RecordExpr{
			Kind:  "RecordExpr",
			Elems: elems,
		}
	case *ast.OverExpr:
		exprs := a.semExprs(e.Exprs)
		if e.Body == nil {
			a.error(e, errors.New("over expression missing lateral scope"))
			return badExpr()
		}
		a.enterScope()
		defer a.exitScope()
		return &dag.OverExpr{
			Kind:  "OverExpr",
			Defs:  a.semVars(e.Locals),
			Exprs: exprs,
			Body:  a.semSeq(e.Body),
		}
	case *ast.FString:
		return a.semFString(e)
	case *ast.CaseExpr:
		a.error(e, errors.New("case matching-style expressions not yet supported"))
		return badExpr()
	}
	panic(errors.New("invalid expression type"))
}

func (a *analyzer) semID(id *ast.ID) dag.Expr {
	// We use static scoping here to see if an identifier is
	// a "var" reference to the name or a field access
	// and transform the AST node appropriately.  The resulting DAG
	// doesn't have Identifiers as they are resolved here
	// one way or the other.
	ref, err := a.scope.LookupExpr(id.Name)
	if err != nil {
		a.error(id, err)
		return badExpr()
	}
	if ref != nil {
		return ref
	}
	return pathOf(id.Name)
}

func semDynamicType(tv ast.Type) *dag.Call {
	if typeName, ok := tv.(*ast.TypeName); ok {
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

func (a *analyzer) semGrep(grep *ast.Grep) dag.Expr {
	e := dag.Expr(&dag.This{Kind: "This"})
	if grep.Expr != nil {
		e = a.semExpr(grep.Expr)
	}
	p := a.semExpr(grep.Pattern)
	if s, ok := p.(*dag.RegexpSearch); ok {
		s.Expr = e
		return s
	}
	if s, ok := isStringConst(a.zctx, p); ok {
		return &dag.Search{
			Kind:  "Search",
			Text:  s,
			Value: zson.QuotedString([]byte(s)),
			Expr:  e,
		}
	}
	return &dag.Call{
		Kind: "Call",
		Name: "grep",
		Args: []dag.Expr{p, e},
	}
}

func (a *analyzer) semRegexp(b *ast.BinaryExpr) dag.Expr {
	if b.Op != "~" {
		return nil
	}
	re, ok := b.RHS.(*ast.Regexp)
	if !ok {
		a.error(b, errors.New(`right-hand side of ~ expression must be a regular expression`))
		return badExpr()
	}
	if _, err := expr.CompileRegexp(re.Pattern); err != nil {
		a.error(b.RHS, err)
		return badExpr()
	}
	e := a.semExpr(b.LHS)
	return &dag.RegexpMatch{
		Kind:    "RegexpMatch",
		Pattern: re.Pattern,
		Expr:    e,
	}
}

func (a *analyzer) semBinary(e *ast.BinaryExpr) dag.Expr {
	if path := a.semDotted(e); path != nil {
		if a.scope.schema != nil {
			var err error
			path, err = a.scope.resolve(path)
			if err != nil {
				a.error(e, err)
				return badExpr()
			}
		}
		return &dag.This{Kind: "This", Path: path}
	}
	if e := a.semRegexp(e); e != nil {
		return e
	}
	op := strings.ToLower(e.Op)
	if op == "." {
		lhs := a.semExpr(e.LHS)
		id, ok := e.RHS.(*ast.ID)
		if !ok {
			a.error(e, errors.New("RHS of dot operator is not an identifier"))
			return badExpr()
		}
		if lhs, ok := lhs.(*dag.This); ok {
			lhs.Path = append(lhs.Path, id.Name)
			return lhs
		}
		return &dag.Dot{
			Kind: "Dot",
			LHS:  lhs,
			RHS:  id.Name,
		}
	}
	lhs := a.semExpr(e.LHS)
	rhs := a.semExpr(e.RHS)
	if op == "like" {
		s, ok := isStringConst(a.zctx, rhs)
		if !ok {
			a.error(e.RHS, errors.New("non-constant pattern for LIKE not supported"))
			return badExpr()
		}
		pattern := likeexpr.ToRegexp(s, '\\', false)
		return &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: "(?s)" + pattern,
			Expr:    lhs,
		}
	}
	switch op {
	case "=":
		op = "=="
	case "<>":
		op = "!="
	case "||":
		op = "+"
	}
	return &dag.BinaryExpr{
		Kind: "BinaryExpr",
		Op:   op,
		LHS:  lhs,
		RHS:  rhs,
	}
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

func isStringConst(zctx *super.Context, e dag.Expr) (field string, ok bool) {
	val, err := kernel.EvalAtCompileTime(zctx, e)
	if err == nil && !val.IsError() && super.TypeUnder(val.Type()) == super.TypeString {
		return string(val.Bytes()), true
	}
	return "", false
}

func (a *analyzer) semExprNullable(e ast.Expr) dag.Expr {
	if e == nil {
		return nil
	}
	return a.semExpr(e)
}

func (a *analyzer) semDotted(e *ast.BinaryExpr) []string {
	if e.Op != "." {
		return nil
	}
	rhs, ok := e.RHS.(*ast.ID)
	if !ok {
		return nil
	}
	switch lhs := e.LHS.(type) {
	case *ast.ID:
		id := a.semID(lhs)
		if this, ok := id.(*dag.This); ok {
			return append(slices.Clone(this.Path), rhs.Name)
		}
		return nil
	case *ast.BinaryExpr:
		this := a.semDotted(lhs)
		if this == nil {
			return nil
		}
		return append(this, rhs.Name)
	}
	return nil
}

func (a *analyzer) semCall(call *ast.Call) dag.Expr {
	if e := a.maybeConvertAgg(call); e != nil {
		return e
	}
	if call.Where != nil {
		a.error(call, errors.New("'where' clause on non-aggregation function"))
		return badExpr()
	}
	exprs := a.semExprs(call.Args)
	name, nargs := call.Name.Name, len(call.Args)
	nameLower := strings.ToLower(name)
	// Call could be to a user defined func. Check if we have a matching func in
	// scope.
	udf, err := a.scope.LookupExpr(name)
	if err != nil {
		a.error(call, err)
		return badExpr()
	}
	switch {
	// udf should be checked first since a udf can override builtin functions.
	case udf != nil:
		f, ok := udf.(*dag.Func)
		if !ok {
			a.error(call.Name, errors.New("not a function"))
			return badExpr()
		}
		if len(f.Params) != nargs {
			a.error(call, fmt.Errorf("call expects %d argument(s)", len(f.Params)))
			return badExpr()
		}
		return &dag.Call{
			Kind: "Call",
			Name: name,
			Args: exprs,
		}
	case super.LookupPrimitive(name) != nil:
		// Primitive function call, change this to a cast.
		if err := function.CheckArgCount(nargs, 1, 1); err != nil {
			a.error(call, err)
			return badExpr()
		}
		exprs = append(exprs, &dag.Literal{Kind: "Literal", Value: "<" + name + ">"})
		name = "cast"
		nameLower = name
	case expr.NewShaperTransform(nameLower) != 0:
		if err := function.CheckArgCount(nargs, 1, 2); err != nil {
			a.error(call, err)
			return badExpr()
		}
		if nargs == 1 {
			exprs = append([]dag.Expr{&dag.This{Kind: "This"}}, exprs...)
		}
	case nameLower == "map":
		if err := function.CheckArgCount(nargs, 2, 2); err != nil {
			a.error(call, err)
			return badExpr()
		}
		id, ok := call.Args[1].(*ast.ID)
		if !ok {
			a.error(call.Args[1], errors.New("second argument must be the identifier of a function"))
			return badExpr()
		}
		inner := a.semCall(&ast.Call{
			Kind: "Call",
			Name: id,
			Args: []ast.Expr{&ast.ID{Kind: "ID", Name: "this"}},
		})
		return &dag.MapCall{
			Kind:  "MapCall",
			Expr:  exprs[0],
			Inner: inner,
		}
	default:
		if _, _, err = function.New(a.zctx, nameLower, nargs); err != nil {
			a.error(call, err)
			return badExpr()
		}
	}
	return &dag.Call{
		Kind: "Call",
		Name: nameLower,
		Args: exprs,
	}
}

func (a *analyzer) semExprs(in []ast.Expr) []dag.Expr {
	exprs := make([]dag.Expr, 0, len(in))
	for _, e := range in {
		exprs = append(exprs, a.semExpr(e))
	}
	return exprs
}

func (a *analyzer) semAssignments(assignments []ast.Assignment) []dag.Assignment {
	out := make([]dag.Assignment, 0, len(assignments))
	for _, e := range assignments {
		out = append(out, a.semAssignment(e))
	}
	return out
}

func (a *analyzer) semAssignment(assign ast.Assignment) dag.Assignment {
	rhs := a.semExpr(assign.RHS)
	var lhs dag.Expr
	if assign.LHS == nil {
		if path, err := deriveLHSPath(rhs); err != nil {
			a.error(&assign, err)
			lhs = badExpr()
		} else {
			lhs = &dag.This{Kind: "This", Path: path}
		}
	} else {
		lhs = a.semExpr(assign.LHS)
	}
	if !isLval(lhs) {
		a.error(&assign, errors.New("illegal left-hand side of assignment"))
		lhs = badExpr()
	}
	if this, ok := lhs.(*dag.This); ok && len(this.Path) == 0 {
		a.error(&assign, errors.New("cannot assign to 'this'"))
		lhs = badExpr()
	}
	return dag.Assignment{Kind: "Assignment", LHS: lhs, RHS: rhs}
}

func isLval(e dag.Expr) bool {
	switch e := e.(type) {
	case *dag.IndexExpr:
		return isLval(e.Expr)
	case *dag.Dot:
		return isLval(e.LHS)
	case *dag.This:
		return true
	}
	return false
}

func deriveLHSPath(rhs dag.Expr) ([]string, error) {
	switch rhs := rhs.(type) {
	case *dag.Agg:
		return []string{rhs.Name}, nil
	case *dag.Call:
		switch strings.ToLower(rhs.Name) {
		case "every":
			// If LHS is nil and the call is every() make the LHS field ts since
			// field ts assumed with every.
			return []string{"ts"}, nil
		case "quiet":
			if len(rhs.Args) > 0 {
				if this, ok := rhs.Args[0].(*dag.This); ok {
					return this.Path, nil
				}
			}
		}
		return []string{rhs.Name}, nil
	case *dag.This:
		return rhs.Path, nil
	case *dag.Var:
		return []string{rhs.Name}, nil
	}
	return nil, errors.New("cannot infer field from expression")
}

func (a *analyzer) semFields(exprs []ast.Expr) []dag.Expr {
	fields := make([]dag.Expr, 0, len(exprs))
	for _, e := range exprs {
		fields = append(fields, a.semField(e))
	}
	return fields
}

// semField analyzes the expression f and makes sure that it's
// a field reference returning an error if not.
func (a *analyzer) semField(f ast.Expr) dag.Expr {
	e := a.semExpr(f)
	field, ok := e.(*dag.This)
	if !ok {
		a.error(f, errors.New("invalid expression used as a field"))
		return badExpr()
	}
	if len(field.Path) == 0 {
		a.error(f, errors.New("cannot use 'this' as a field reference"))
		return badExpr()
	}
	return field
}

func (a *analyzer) maybeConvertAgg(call *ast.Call) dag.Expr {
	name := call.Name.Name
	nameLower := strings.ToLower(name)
	if _, err := agg.NewPattern(nameLower, true); err != nil {
		return nil
	}
	var e dag.Expr
	if err := function.CheckArgCount(len(call.Args), 0, 1); err != nil {
		if nameLower == "min" || nameLower == "max" {
			// min and max are special cases as they are also functions. If the
			// number of args is greater than 1 they're probably a function so do not
			// return an error.
			return nil
		}
		a.error(call, err)
		return badExpr()
	}
	if len(call.Args) == 1 {
		e = a.semExpr(call.Args[0])
	}
	return &dag.Agg{
		Kind:  "Agg",
		Name:  nameLower,
		Expr:  e,
		Where: a.semExprNullable(call.Where),
	}
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
	case *ast.IndexExpr:
		this := DotExprToFieldPath(e.Expr)
		if this == nil {
			return nil
		}
		id, ok := e.Index.(*ast.Primitive)
		if !ok || id.Type != "string" {
			return nil
		}
		this.Path = append(this.Path, id.Text)
		return this
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

func (a *analyzer) semType(typ ast.Type) (string, error) {
	ztype, err := zson.TranslateType(a.zctx, typ)
	if err != nil {
		return "", err
	}
	return zson.FormatType(ztype), nil
}

func (a *analyzer) semVectorElems(elems []ast.VectorElem) []dag.VectorElem {
	var out []dag.VectorElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *ast.Spread:
			e := a.semExpr(elem.Expr)
			out = append(out, &dag.Spread{Kind: "Spread", Expr: e})
		case *ast.VectorValue:
			e := a.semExpr(elem.Expr)
			out = append(out, &dag.VectorValue{Kind: "VectorValue", Expr: e})
		}
	}
	return out
}

func (a *analyzer) semFString(f *ast.FString) dag.Expr {
	if len(f.Elems) == 0 {
		return &dag.Literal{Kind: "Literal", Value: `""`}
	}
	var out dag.Expr
	for _, elem := range f.Elems {
		var e dag.Expr
		switch elem := elem.(type) {
		case *ast.FStringExpr:
			e = a.semExpr(elem.Expr)
			e = &dag.Call{
				Kind: "Call",
				Name: "cast",
				Args: []dag.Expr{e, &dag.Literal{Kind: "Literal", Value: "<string>"}},
			}
		case *ast.FStringText:
			e = &dag.Literal{Kind: "Literal", Value: zson.QuotedString([]byte(elem.Text))}
		default:
			panic(fmt.Errorf("internal error: unsupported f-string elem %T", elem))
		}
		if out == nil {
			out = e
			continue
		}
		out = &dag.BinaryExpr{Kind: "BinaryExpr", LHS: out, Op: "+", RHS: e}
	}
	return out
}
