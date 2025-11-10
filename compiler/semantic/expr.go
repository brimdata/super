package semantic

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/araddon/dateparse"
	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/compiler/sfmt"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/pkg/reglob"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/sup"
	"github.com/shellyln/go-sql-like-expr/likeexpr"
)

func (t *translator) expr(e ast.Expr) sem.Expr {
	switch e := e.(type) {
	case *ast.Agg:
		expr := t.exprNullable(e.Expr)
		nameLower := strings.ToLower(e.Name)
		if expr == nil && nameLower != "count" {
			t.error(e, fmt.Errorf("aggregator '%s' requires argument", e.Name))
			return badExpr()
		}
		return t.aggFunc(e, nameLower, e.Expr, e.Where, e.Distinct)
	case *ast.ArrayExpr:
		elems := t.arrayElems(e.Elems)
		if subquery := t.arraySubquery(elems); subquery != nil {
			return subquery
		}
		return &sem.ArrayExpr{
			Node:  e,
			Elems: elems,
		}
	case *ast.BinaryExpr:
		return t.binaryExpr(e)
	case *ast.BetweenExpr:
		val := t.expr(e.Expr)
		lower := t.expr(e.Lower)
		upper := t.expr(e.Upper)
		// Copy val so an optimizer change to one instance doesn't affect the other.
		expr := &sem.BinaryExpr{
			Node: e,
			Op:   "and",
			LHS: &sem.BinaryExpr{
				Node: e.Lower,
				Op:   ">=",
				LHS:  val,
				RHS:  lower,
			},
			RHS: &sem.BinaryExpr{
				Node: e.Upper,
				Op:   "<=",
				LHS:  val,
				RHS:  upper,
			},
		}
		if e.Not {
			return sem.NewUnaryExpr(e, "!", expr)
		}
		return expr
	case *ast.CaseExpr:
		return t.semCaseExpr(e)
	case *ast.CondExpr:
		cond := t.expr(e.Cond)
		thenExpr := t.expr(e.Then)
		var elseExpr sem.Expr
		if e.Else != nil {
			elseExpr = t.expr(e.Else)
		} else {
			elseExpr = &sem.LiteralExpr{Node: e, Value: `error("missing")`}
		}
		return &sem.CondExpr{
			Node: e,
			Cond: cond,
			Then: thenExpr,
			Else: elseExpr,
		}
	case *ast.CallExpr:
		return t.semCall(e)
	case *ast.CastExpr:
		expr := t.expr(e.Expr)
		if _, ok := e.Type.(*ast.DateTypeHack); ok {
			// cast to time then bucket by 1d as a workaround for not currently
			// supporting a "date" type.
			cast := sem.NewCall(e, "cast", []sem.Expr{expr, &sem.LiteralExpr{Node: e, Value: "<time>"}})
			return sem.NewCall(e, "bucket", []sem.Expr{cast, &sem.LiteralExpr{Node: e, Value: "1d"}})
		}
		typ := t.expr(&ast.TypeValue{
			Kind:  "TypeValue",
			Value: e.Type,
		})
		return sem.NewCall(e, "cast", []sem.Expr{expr, typ})
	case *ast.ExtractExpr:
		return t.semExtractExpr(e, e.Part, e.Expr)
	case *ast.DoubleQuoteExpr:
		return t.doubleQuoteExpr(e)
	case *ast.ExistsExpr:
		return t.existsExpr(e)
	case *ast.FStringExpr:
		return t.fstringExpr(e)
	case *ast.FuncNameExpr:
		// We get here for &refs that are in a call expression. e.g.,
		// an arg to another function.  It could be a built-in (as in &upper),
		// or a user function (as in fn foo():... &foo)...
		id := e.Name
		if boundID, _ := t.scope.lookupFuncDeclOrParam(id); boundID != "" {
			id = boundID
		}
		return &sem.FuncRef{
			Node: e,
			ID:   id,
		}
	case *ast.GlobExpr:
		return &sem.RegexpSearchExpr{
			Node:    e,
			Pattern: reglob.Reglob(e.Pattern),
			Expr:    sem.NewThis(e, nil),
		}
	case *ast.IDExpr:
		id := t.idExpr(e, false)
		if t.scope.schema != nil {
			if this, ok := id.(*sem.ThisExpr); ok {
				ref, err := t.scope.resolve(e, this.Path)
				if err != nil {
					t.error(e, err)
					return badExpr()
				}
				return ref
			}
		}
		return id
	case *ast.IndexExpr:
		expr := t.expr(e.Expr)
		index := t.expr(e.Index)
		// If expr is a path and index is a string, then just extend the path.
		if path := t.isIndexOfThis(expr, index); path != nil {
			return path
		}
		return &sem.IndexExpr{
			Node:  e,
			Expr:  expr,
			Index: index,
			Base1: t.scope.indexBase() == 1,
		}
	case *ast.IsNullExpr:
		expr := t.expr(e.Expr)
		var out sem.Expr = &sem.IsNullExpr{Node: e, Expr: expr}
		if e.Not {
			out = sem.NewUnaryExpr(e, "!", out)
		}
		return out
	case *ast.LambdaExpr:
		funcDecl := t.resolver.newFuncDecl("lambda", e, t.scope)
		return &sem.FuncRef{
			Node: e,
			ID:   funcDecl.id,
		}
	case *ast.MapExpr:
		var entries []sem.Entry
		for _, entry := range e.Entries {
			key := t.expr(entry.Key)
			val := t.expr(entry.Value)
			entries = append(entries, sem.Entry{Key: key, Value: val})
		}
		return &sem.MapExpr{
			Node:    e,
			Entries: entries,
		}
	case *ast.Primitive:
		val, err := sup.ParsePrimitive(e.Type, e.Text)
		if err != nil {
			t.error(e, err)
			return badExpr()
		}
		return &sem.LiteralExpr{
			Node:  e,
			Value: sup.FormatValue(val),
		}
	case *ast.SubqueryExpr:
		return t.subqueryExpr(e, e.Array, e.Body)
	case *ast.RecordExpr:
		fields := map[string]struct{}{}
		var out []sem.RecordElem
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *ast.FieldElem:
				if _, ok := fields[elem.Name.Text]; ok {
					t.error(elem, fmt.Errorf("record expression: %w", &super.DuplicateFieldError{Name: elem.Name.Text}))
					continue
				}
				fields[elem.Name.Text] = struct{}{}
				e := t.expr(elem.Value)
				out = append(out, &sem.FieldElem{
					Node:  elem,
					Name:  elem.Name.Text,
					Value: e,
				})
			case *ast.SpreadElem:
				e := t.expr(elem.Expr)
				out = append(out, &sem.SpreadElem{
					Node: elem,
					Expr: e,
				})
			case *ast.ExprElem:
				e := t.expr(elem.Expr)
				name := deriveNameFromExpr(elem.Expr)
				if _, ok := fields[name]; ok {
					t.error(elem, fmt.Errorf("record expression: %w", &super.DuplicateFieldError{Name: name}))
					continue
				}
				fields[name] = struct{}{}
				out = append(out, &sem.FieldElem{
					Name:  name,
					Value: e,
				})
			default:
				panic(e)
			}
		}
		return &sem.RecordExpr{
			Node:  e,
			Elems: out,
		}
	case *ast.RegexpExpr:
		return &sem.RegexpSearchExpr{
			Node:    e,
			Pattern: e.Pattern,
			Expr:    sem.NewThis(e, nil),
		}
	case *ast.SetExpr:
		elems := t.arrayElems(e.Elems)
		return &sem.SetExpr{
			Node:  e,
			Elems: elems,
		}
	case *ast.SliceExpr:
		expr := t.expr(e.Expr)
		// XXX Literal indices should be type checked as int.
		from := t.exprNullable(e.From)
		to := t.exprNullable(e.To)
		return &sem.SliceExpr{
			Node:  e,
			Expr:  expr,
			From:  from,
			To:    to,
			Base1: t.scope.indexBase() == 1,
		}
	case *ast.SQLTimeExpr:
		if e.Value.Type != "string" {
			t.error(e.Value, errors.New("value must be a string literal"))
			return badExpr()
		}
		tm, err := dateparse.ParseAny(e.Value.Text)
		if err != nil {
			t.error(e.Value, err)
			return badExpr()
		}
		ts := nano.TimeToTs(tm)
		if e.Type == "date" {
			ts = ts.Trunc(nano.Day)
		}
		return &sem.LiteralExpr{Node: e, Value: sup.FormatValue(super.NewTime(ts))}
	case *ast.SearchTermExpr:
		var val string
		switch term := e.Value.(type) {
		case *ast.Primitive:
			v, err := sup.ParsePrimitive(term.Type, term.Text)
			if err != nil {
				t.error(e, err)
				return badExpr()
			}
			val = sup.FormatValue(v)
		case *ast.DoubleQuoteExpr:
			v, err := sup.ParsePrimitive("string", term.Text)
			if err != nil {
				t.error(e, err)
				return badExpr()
			}
			val = sup.FormatValue(v)
		case *ast.TypeValue:
			tv, err := t.semType(term.Value)
			if err != nil {
				t.error(e, err)
				return badExpr()
			}
			val = "<" + tv + ">"
		default:
			panic(fmt.Errorf("unexpected term value: %s (%T)", e.Kind, e))
		}
		return &sem.SearchTermExpr{
			Node:  e,
			Text:  e.Text,
			Value: val,
			Expr:  sem.NewThis(e, nil),
		}
	case *ast.SubstringExpr:
		expr := t.expr(e.Expr)
		if e.From == nil && e.For == nil {
			t.error(e, errors.New("FROM or FOR must be set"))
			return badExpr()
		}
		// XXX type checker should remove this check when it finds it redundant
		is := sem.NewCall(e, "is", []sem.Expr{expr, &sem.LiteralExpr{Node: e.Expr, Value: "<string>"}})
		slice := &sem.SliceExpr{
			Node:  e,
			Expr:  expr,
			From:  t.exprNullable(e.From),
			Base1: true,
		}
		if e.For != nil {
			to := t.expr(e.For)
			if slice.From != nil {
				slice.To = sem.NewBinaryExpr(e, "+", slice.From, to)
			} else {
				slice.To = sem.NewBinaryExpr(e, "+", to, &sem.LiteralExpr{Node: e, Value: "1"})
			}
		}
		serr := sem.NewStructuredError(e, "SUBSTRING: string value required", expr)
		return &sem.CondExpr{
			Node: e,
			Cond: is,
			Then: slice,
			Else: serr,
		}
	case *ast.TupleExpr:
		elems := make([]sem.RecordElem, 0, len(e.Elems))
		for colno, elem := range e.Elems {
			e := t.expr(elem)
			elems = append(elems, &sem.FieldElem{
				Node:  elem,
				Name:  fmt.Sprintf("c%d", colno),
				Value: e,
			})
		}
		return &sem.RecordExpr{
			Node:  e,
			Elems: elems,
		}
	case *ast.TypeValue:
		typ, err := t.semType(e.Value)
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
			if e := semDynamicType(e, e.Value); e != nil {
				return e
			}
			t.error(e, err)
			return badExpr()
		}
		return &sem.LiteralExpr{
			Node:  e,
			Value: "<" + typ + ">",
		}
	case *ast.UnaryExpr:
		operand := t.expr(e.Operand)
		if e.Op == "+" {
			return operand
		}
		return sem.NewUnaryExpr(e, e.Op, operand)
	case nil:
		panic("semantic analysis: illegal null value encountered in AST")
	}
	panic(e)
}

func (t *translator) idExpr(id *ast.IDExpr, lval bool) sem.Expr {
	// We use static scoping here to see if an identifier is
	// a "var" reference to the name or a field access
	// and transform the AST node appropriately.  The resulting
	// sem tree doesn't have Identifiers as they are resolved here
	// one way or the other.
	if subquery := t.maybeSubquery(id, id.Name); subquery != nil {
		return subquery
	}
	// Check if there's a user function in scope with this name and report
	// an error to avoid a rake when such a function is mistakenly passed
	// without "&" and otherwise turns into a field reference.
	if entry := t.scope.lookupEntry(id.Name); entry != nil {
		if _, ok := entry.ref.(*funcDef); ok && !lval {
			t.error(id, fmt.Errorf("function %q referenced but not called (consider &%s to create a function value)", id.Name, id.Name))
			return badExpr()
		}
	}
	if ref := t.scope.lookupExpr(t, id.Name); ref != nil {
		return ref
	}
	var path []string
	if id.Name != "this" {
		path = []string{id.Name}
	}
	return sem.NewThis(id, path)
}

func (t *translator) doubleQuoteExpr(d *ast.DoubleQuoteExpr) sem.Expr {
	// Check if there's a SQL scope and treat a double-quoted string
	// as an identifier.  XXX we'll need to do something a bit more
	// sophisticated to handle pipes inside SQL subqueries.
	if t.scope.schema != nil {
		if d.Text == "this" {
			ref, err := t.scope.resolve(d, []string{"this"})
			if err != nil {
				t.error(d, err)
				return badExpr()
			}
			return ref
		}
		return t.expr(&ast.IDExpr{Kind: "IDExpr", ID: ast.ID{Name: d.Text, Loc: d.Loc}})
	}
	return t.expr(&ast.Primitive{
		Kind: "Primitive",
		Type: "string",
		Text: d.Text,
		Loc:  d.Loc,
	})
}

func (t *translator) existsExpr(e *ast.ExistsExpr) sem.Expr {
	q := t.subqueryExpr(e, true, e.Body)
	return sem.NewBinaryExpr(e, ">",
		sem.NewCall(e, "len", []sem.Expr{q}),
		&sem.LiteralExpr{Node: e, Value: "0"})
}

func semDynamicType(n ast.Node, tv ast.Type) *sem.CallExpr {
	if typeName, ok := tv.(*ast.TypeName); ok {
		return dynamicTypeName(n, typeName.Name)
	}
	return nil
}

func dynamicTypeName(n ast.Node, name string) *sem.CallExpr {
	return sem.NewCall(
		n,
		"typename",
		[]sem.Expr{
			// SUP string literal of type name
			&sem.LiteralExpr{
				Node:  n,
				Value: `"` + name + `"`,
			},
		},
	)
}

func (t *translator) regexp(b *ast.BinaryExpr) sem.Expr {
	if b.Op != "~" {
		return nil
	}
	s, ok := t.mustEvalString(t.expr(b.RHS))
	if !ok {
		t.error(b, errors.New(`right-hand side of ~ expression must be a string literal`))
		return badExpr()
	}
	if _, err := expr.CompileRegexp(s); err != nil {
		t.error(b.RHS, err)
		return badExpr()
	}
	e := t.expr(b.LHS)
	return &sem.RegexpMatchExpr{
		Node:    b,
		Pattern: s,
		Expr:    e,
	}
}

func (t *translator) binaryExpr(e *ast.BinaryExpr) sem.Expr {
	if path, bad := t.semDotted(e, false); path != nil {
		if t.scope.schema != nil {
			ref, err := t.scope.resolve(e, path)
			if err != nil {
				t.error(e, err)
				return badExpr()
			}
			return ref
		}
		return sem.NewThis(e, path)
	} else if bad != nil {
		return bad
	}
	if e := t.regexp(e); e != nil {
		return e
	}
	op := strings.ToLower(e.Op)
	if op == "." {
		lhs := t.expr(e.LHS)
		id, ok := e.RHS.(*ast.IDExpr)
		if !ok {
			t.error(e, errors.New("RHS of dot operator is not an identifier"))
			return badExpr()
		}
		if lhs, ok := lhs.(*sem.ThisExpr); ok {
			lhs.Path = append(lhs.Path, id.Name)
			return lhs
		}
		return &sem.DotExpr{
			Node: e,
			LHS:  lhs,
			RHS:  id.Name,
		}
	}
	lhs := t.expr(e.LHS)
	rhs := t.expr(e.RHS)
	if op == "like" || op == "not like" {
		s, ok := t.mustEvalString(rhs)
		if !ok {
			t.error(e.RHS, errors.New("non-constant pattern for LIKE not supported"))
			return badExpr()
		}
		pattern := likeexpr.ToRegexp(s, '\\', false)
		expr := &sem.RegexpSearchExpr{
			Node:    e,
			Pattern: "(?s)" + pattern,
			Expr:    lhs,
		}
		if op == "not like" {
			return &sem.UnaryExpr{
				Node:    e,
				Op:      "!",
				Operand: expr,
			}
		}
		return expr
	}
	if op == "in" || op == "not in" {
		if q, ok := rhs.(*sem.SubqueryExpr); ok {
			q.Array = true
		}
	}
	switch op {
	case "=":
		op = "=="
	case "<>":
		op = "!="
	case "||":
		op = "+"
	case "not in":
		return sem.NewUnaryExpr(e, "!", sem.NewBinaryExpr(e, "in", lhs, rhs))
	case "::":
		return &sem.CallExpr{
			Node: e,
			Tag:  "cast",
			Args: []sem.Expr{lhs, rhs},
		}
	}
	return &sem.BinaryExpr{
		Node: e,
		Op:   op,
		LHS:  lhs,
		RHS:  rhs,
	}
}

func (t *translator) isIndexOfThis(lhs, rhs sem.Expr) *sem.ThisExpr {
	if this, ok := lhs.(*sem.ThisExpr); ok {
		if s, ok := t.maybeEvalString(rhs); ok {
			this.Path = append(this.Path, s)
			return this
		}
	}
	return nil
}

func (t *translator) exprNullable(e ast.Expr) sem.Expr {
	if e == nil {
		return nil
	}
	return t.expr(e)
}

func (t *translator) semDotted(e *ast.BinaryExpr, lval bool) ([]string, sem.Expr) {
	if e.Op != "." {
		return nil, nil
	}
	rhs, ok := e.RHS.(*ast.IDExpr)
	if !ok {
		return nil, nil
	}
	switch lhs := e.LHS.(type) {
	case *ast.IDExpr:
		switch e := t.idExpr(lhs, lval).(type) {
		case *sem.ThisExpr:
			return append(slices.Clone(e.Path), rhs.Name), nil
		case *sem.BadExpr:
			return nil, e
		default:
			return nil, nil
		}
	case *ast.BinaryExpr:
		this, bad := t.semDotted(lhs, lval)
		if this == nil {
			return nil, bad
		}
		return append(this, rhs.Name), nil
	}
	return nil, nil
}

func (t *translator) semCaseExpr(c *ast.CaseExpr) sem.Expr {
	e := t.expr(c.Expr)
	out := t.exprNullable(c.Else)
	for i := len(c.Whens) - 1; i >= 0; i-- {
		when := c.Whens[i]
		out = &sem.CondExpr{
			Node: c,
			Cond: sem.NewBinaryExpr(c, "==", e, t.expr(when.Cond)),
			Then: t.expr(when.Then),
			Else: out,
		}
	}
	return out
}

func (t *translator) semCall(call *ast.CallExpr) sem.Expr {
	if e := t.maybeConvertAgg(call); e != nil {
		return e
	}
	if call.Where != nil {
		t.error(call, errors.New("'where' clause on non-aggregation function"))
		return badExpr()
	}
	args := t.exprs(call.Args)
	switch f := call.Func.(type) {
	case *ast.FuncNameExpr:
		return t.semCallByName(call, f.Name, args)
	case *ast.LambdaExpr:
		return t.semCallLambda(f, args)
	default:
		panic(f)
	}
}

func (t *translator) maybeSubquery(n ast.Node, name string) *sem.SubqueryExpr {
	if seq := t.scope.lookupQuery(t, name); seq != nil {
		return &sem.SubqueryExpr{
			Node:       n,
			Correlated: isCorrelated(seq),
			Body:       seq,
		}
	}
	return nil
}

func (t *translator) semCallLambda(lambda *ast.LambdaExpr, args []sem.Expr) sem.Expr {
	funcDecl := t.resolver.newFuncDecl("lambda", lambda, t.scope)
	return t.resolver.mustResolveCall(lambda, funcDecl.id, args)
}

func (t *translator) semCallByName(call *ast.CallExpr, name string, args []sem.Expr) sem.Expr {
	if subquery := t.maybeSubqueryCall(call, name); subquery != nil {
		return subquery
	}
	// Check if the name resolves to a symbol in scope.
	if entry := t.scope.lookupEntry(name); entry != nil {
		switch ref := entry.ref.(type) {
		case funcParamValue:
			t.error(call, fmt.Errorf("function called via parameter %q is bound to a non-function", name))
			return badExpr()
		case *funcParamLambda:
			// Called name is a parameter inside of a function.   We only end up here
			// when actual values have been bound to the parameter (i.e., we're compiling
			// a lambda-variant function each time it is called to create each variant),
			// so we call the resolver here to create a new instance of the function being
			// called. In the case of recursion, all the lambdas that are further passed
			// as args are known (in terms of their decl IDs), so the resolver can
			// look this up in the variants of the decl and stop the recursion even if the body
			// of the called entity is not completed yet.  We won't know the type but we
			// can't know the type without function type signatures so when we integrate
			// type checking here, we will use unknown for this corner case.
			if isBuiltin(ref.id) {
				// Check argument count here for builtin functions.
				if _, err := function.New(super.NewContext(), ref.id, len(args)); err != nil {
					t.error(call, fmt.Errorf("function %q called via parameter %q: %w", ref.id, ref.param, err))
					return badExpr()
				}
				return sem.NewCall(call, ref.id, args)
			}
			return t.resolver.mustResolveCall(call, ref.id, args)
		case *opDecl:
			t.error(call, fmt.Errorf("cannot call user operator %q in an expression (consider subquery syntax)", name))
			return badExpr()
		case *sem.FuncRef:
			// FuncRefs are put in the symbol table when passing stuff to user ops, e.g.,
			// a lambda as a parameter, a &func, or a builtin like &upper.
			return t.resolver.mustResolveCall(ref, ref.ID, args)
		case *funcDecl:
			return t.resolver.mustResolveCall(call, ref.id, args)
		case *constDecl, *queryDecl:
			t.error(call, fmt.Errorf("%q is not a function", name))
			return badExpr()
		}
		if _, ok := entry.ref.(sem.Expr); ok {
			t.error(call, fmt.Errorf("%q is not a function", name))
			return badExpr()
		}
		panic(entry.ref)
	}
	nargs := len(args)
	nameLower := strings.ToLower(name)
	switch {
	case nameLower == "map":
		return t.semMapCall(call, args)
	case nameLower == "grep":
		if err := function.CheckArgCount(nargs, 2, 2); err != nil {
			t.error(call, err)
			return badExpr()
		}
		pattern, ok := t.maybeEvalString(args[0])
		if !ok {
			return sem.NewCall(call, "grep", args)
		}
		re, err := expr.CompileRegexp(pattern)
		if err != nil {
			t.error(call.Args[0], err)
			return badExpr()
		}
		if s, ok := re.LiteralPrefix(); ok {
			return &sem.SearchTermExpr{
				Node:  call,
				Text:  s,
				Value: sup.QuotedString(s),
				Expr:  args[1],
			}
		}
		return &sem.RegexpSearchExpr{
			Node:    call,
			Pattern: pattern,
			Expr:    args[1],
		}
	case nameLower == "position" && nargs == 1:
		b, ok := args[0].(*sem.BinaryExpr)
		if ok && strings.ToLower(b.Op) == "in" {
			// Support for SQL style position function call.
			args = []sem.Expr{b.RHS, b.LHS}
			nargs = 2
		}
		fallthrough
	default:
		if _, err := function.New(t.sctx, nameLower, nargs); err != nil {
			t.error(call, err)
			return badExpr()
		}
	}
	return sem.NewCall(call, nameLower, args)
}

func (t *translator) maybeSubqueryCall(call *ast.CallExpr, name string) *sem.SubqueryExpr {
	decl, _ := t.scope.lookupOp(name)
	if decl == nil || decl.bad {
		return nil
	}
	userOp := t.userOp(call.Loc, decl, call.Args)
	// When a user op is encountered inside an expression, we turn it into a
	// subquery operating on a single-shot "this" value unless it's uncorrelated
	// (i.e., starts with a from), in which case we put the uncorrelated body
	// in the subquery without the correlating values logic. This is controlled
	// via the Correlated flag.
	return &sem.SubqueryExpr{
		Node:       call,
		Array:      false,
		Correlated: isCorrelated(userOp),
		Body:       userOp,
	}
}

func (t *translator) semMapCall(call *ast.CallExpr, args []sem.Expr) sem.Expr {
	if len(args) != 2 {
		t.error(call, errors.New("map requires two arguments"))
		return badExpr()
	}
	ref, ok := args[1].(*sem.FuncRef)
	if !ok {
		t.error(call, errors.New("second argument to map must be a function"))
		return badExpr()
	}
	mapArgs := []sem.Expr{sem.NewThis(call.Args[1], nil)}
	e := t.resolver.resolveCall(call.Args[1], ref.ID, mapArgs)
	if callExpr, ok := e.(*sem.CallExpr); ok {
		return &sem.MapCallExpr{
			Node:   call,
			Expr:   args[0],
			Lambda: callExpr,
		}
	}
	return e
}

func (t *translator) semExtractExpr(e, partExpr, argExpr ast.Expr) sem.Expr {
	var partstr string
	switch p := partExpr.(type) {
	case *ast.IDExpr:
		partstr = p.Name
	case *ast.Primitive:
		if p.Type != "string" {
			t.error(partExpr, fmt.Errorf("part must be an identifier or string"))
			return badExpr()
		} else {
			partstr = p.Text
		}
	default:
		t.error(partExpr, fmt.Errorf("part must be an identifier or string"))
		return badExpr()
	}
	return sem.NewCall(e,
		"date_part",
		[]sem.Expr{
			&sem.LiteralExpr{Node: partExpr, Value: sup.QuotedString(strings.ToLower(partstr))},
			t.expr(argExpr),
		},
	)
}

func (t *translator) exprs(in []ast.Expr) []sem.Expr {
	exprs := make([]sem.Expr, 0, len(in))
	for _, e := range in {
		exprs = append(exprs, t.expr(e))
	}
	return exprs
}

func (t *translator) assignments(assignments []ast.Assignment) []sem.Assignment {
	out := make([]sem.Assignment, 0, len(assignments))
	for _, e := range assignments {
		out = append(out, t.assignment(&e))
	}
	return out
}

func (t *translator) assignment(assign *ast.Assignment) sem.Assignment {
	rhs := t.expr(assign.RHS)
	var lhs sem.Expr
	if assign.LHS == nil {
		lhs = sem.NewThis(assign.RHS, []string{deriveNameFromExpr(assign.RHS)})
	} else {
		lhs = t.lval(assign.LHS)
	}
	if !isLval(lhs) {
		t.error(assign, errors.New("illegal left-hand side of assignment"))
		lhs = badExpr()
	}
	if this, ok := lhs.(*sem.ThisExpr); ok && len(this.Path) == 0 {
		t.error(assign, errors.New("cannot assign to 'this'"))
		lhs = badExpr()
	}
	return sem.Assignment{Node: assign, LHS: lhs, RHS: rhs}
}

func (t *translator) lval(e ast.Expr) sem.Expr {
	if id, ok := e.(*ast.IDExpr); ok {
		return t.idExpr(id, true)
	}
	return t.expr(e)
}

func isLval(e sem.Expr) bool {
	switch e := e.(type) {
	case *sem.IndexExpr:
		return isLval(e.Expr)
	case *sem.DotExpr:
		return isLval(e.LHS)
	case *sem.ThisExpr:
		return true
	}
	return false
}

func deriveNameFromExpr(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Agg:
		return e.Name
	case *ast.CallExpr:
		var name string
		if f, ok := e.Func.(*ast.FuncNameExpr); ok {
			name = f.Name
		}
		if strings.ToLower(name) == "quiet" && len(e.Args) > 0 {
			return deriveNameFromExpr(e.Args[0])
		}
		return name
	case *ast.BinaryExpr:
		if name, ok := dottedName(e); ok {
			return name
		}
	case *ast.DoubleQuoteExpr:
		if s, ok := quoteString(e); ok {
			return s
		}
	case *ast.IDExpr:
		if e.Name == "this" {
			return "that"
		}
		return e.Name
	}
	return sfmt.ASTExpr(e)
}

func dottedName(e *ast.BinaryExpr) (string, bool) {
	if e.Op != "." {
		return "", false
	}
	rhs, ok := idString(e.RHS)
	if !ok {
		return "", false
	}
	if _, ok := idString(e.LHS); ok {
		return rhs, true
	}
	if lhs, ok := e.LHS.(*ast.BinaryExpr); ok {
		if _, ok := dottedName(lhs); ok {
			return rhs, true
		}
	}
	return "", false
}

func idString(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.IDExpr:
		return e.Name, true
	case *ast.DoubleQuoteExpr:
		return quoteString(e)
	}
	return "", false
}

func quoteString(e *ast.DoubleQuoteExpr) (string, bool) {
	v, err := sup.ParsePrimitive("string", e.Text)
	if err == nil {
		return v.AsString(), true
	}
	return "", false
}

func (t *translator) fields(exprs []ast.Expr) []sem.Expr {
	fields := make([]sem.Expr, 0, len(exprs))
	for _, e := range exprs {
		fields = append(fields, t.field(e))
	}
	return fields
}

// semField analyzes the expression f and makes sure that it's
// a field reference returning an error if not.
func (t *translator) field(f ast.Expr) sem.Expr {
	switch e := t.expr(f).(type) {
	case *sem.ThisExpr:
		if len(e.Path) == 0 {
			t.error(f, errors.New("cannot use 'this' as a field reference"))
			return badExpr()
		}
		return e
	case *sem.BadExpr:
		return e
	default:
		t.error(f, errors.New("invalid expression used as a field"))
		return badExpr()
	}
}

func (t *translator) maybeConvertAgg(call *ast.CallExpr) sem.Expr {
	name, ok := call.Func.(*ast.FuncNameExpr)
	if !ok {
		return nil
	}
	nameLower := strings.ToLower(name.Name)
	if _, err := agg.NewPattern(nameLower, false, true); err != nil {
		return nil
	}
	if err := function.CheckArgCount(len(call.Args), 0, 1); err != nil {
		if nameLower == "min" || nameLower == "max" {
			// min and max are special cases as they are also functions. If the
			// number of args is greater than 1 they're probably a function so do not
			// return an error.
			return nil
		}
		t.error(call, err)
		return badExpr()
	}
	var e ast.Expr
	if len(call.Args) == 1 {
		e = call.Args[0]
	}
	return t.aggFunc(call, nameLower, e, call.Where, false)
}

func (t *translator) aggFunc(n ast.Node, name string, arg ast.Expr, where ast.Expr, distinct bool) *sem.AggFunc {
	// If we are in the context of a having clause, re-expose the select schema
	// since the agg func's arguments and where clause operate on the input relation
	// not the output.
	if having, ok := t.scope.schema.(*havingSchema); ok {
		save := t.scope.schema
		t.scope.schema = having.selectSchema
		defer func() {
			t.scope.schema = save
		}()
	}
	return &sem.AggFunc{
		Node:     n,
		Name:     name,
		Expr:     t.exprNullable(arg),
		Where:    t.exprNullable(where),
		Distinct: distinct,
	}
}

func DotExprToFieldPath(e ast.Expr) *sem.ThisExpr {
	switch e := e.(type) {
	case *ast.BinaryExpr:
		if e.Op == "." {
			lhs := DotExprToFieldPath(e.LHS)
			if lhs == nil {
				return nil
			}
			id, ok := e.RHS.(*ast.IDExpr)
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
	case *ast.IDExpr:
		return sem.NewThis(e, []string{e.Name})
	}
	// This includes a null Expr, which can happen if the AST is missing
	// a field or sets it to null.
	return nil
}

func (t *translator) semType(typ ast.Type) (string, error) {
	ztype, err := sup.TranslateType(t.sctx, typ)
	if err != nil {
		return "", err
	}
	return sup.FormatType(ztype), nil
}

func (t *translator) arrayElems(elems []ast.ArrayElem) []sem.ArrayElem {
	var out []sem.ArrayElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *ast.SpreadElem:
			out = append(out, &sem.SpreadElem{Node: elem, Expr: t.expr(elem.Expr)})
		case *ast.ExprElem:
			out = append(out, &sem.ExprElem{Node: elem, Expr: t.expr(elem.Expr)})
		default:
			panic(elem)
		}
	}
	return out
}

func (t *translator) fstringExpr(f *ast.FStringExpr) sem.Expr {
	if len(f.Elems) == 0 {
		return &sem.LiteralExpr{Node: f, Value: `""`}
	}
	var out sem.Expr
	for _, elem := range f.Elems {
		var e sem.Expr
		switch elem := elem.(type) {
		case *ast.FStringExprElem:
			e = t.expr(elem.Expr)
			e = sem.NewCall(f,
				"cast",
				[]sem.Expr{e, &sem.LiteralExpr{Value: "<string>"}})
		case *ast.FStringTextElem:
			e = &sem.LiteralExpr{Value: sup.QuotedString(elem.Text)}
		default:
			panic(elem)
		}
		if out == nil {
			out = e
			continue
		}
		out = sem.NewBinaryExpr(f, "+", out, e)
	}
	return out
}

func (t *translator) arraySubquery(elems []sem.ArrayElem) *sem.SubqueryExpr {
	if len(elems) == 1 {
		if elem, ok := elems[0].(*sem.ExprElem); ok {
			if subquery, ok := elem.Expr.(*sem.SubqueryExpr); ok {
				subquery.Array = true
				return subquery
			}
		}
	}
	return nil
}

func (t *translator) subqueryExpr(astExpr ast.Expr, array bool, body ast.Seq) *sem.SubqueryExpr {
	seq := t.seq(body)
	correlated := isCorrelated(seq)
	e := &sem.SubqueryExpr{
		Node:       astExpr,
		Array:      array,
		Correlated: correlated,
		Body:       seq,
	}
	// Add a nullscan only for uncorrelated queries that don't have a source.
	if !correlated && !HasSource(e.Body) {
		e.Body.Prepend(&sem.NullScan{})
	}
	if !array && t.scope.schema != nil {
		// SQL expects a record with a single column result so unravel this
		// condition with this complex cleanup...
		e.Body.Append(scalarSubqueryCheck(astExpr))
	}
	return e
}

func scalarSubqueryCheck(n ast.Node) *sem.ValuesOp {
	// In a SQL expression (except for RHS of EXISTS or IN, which both use array result),
	// a subquery returns a scalar but the result of the subquery is a relation.
	// The runtime already checks if the subquery returns multiple "rows", so
	// here we just check if the relation is a single column and error appropriately
	// or otherwise pull out the value to make the scalar.
	// values is_error(this) ? this : (len(this) == 1 ? this[1] : error(...) )
	lenCall := &sem.CallExpr{
		Node: n,
		Tag:  "len",
		Args: []sem.Expr{sem.NewThis(n, nil)},
	}
	lenCond := &sem.BinaryExpr{
		Node: n,
		Op:   "==",
		LHS:  lenCall,
		RHS:  &sem.LiteralExpr{Node: n, Value: "1"},
	}
	indexExpr := &sem.IndexExpr{
		Node:  n,
		Expr:  sem.NewThis(n, nil),
		Index: &sem.LiteralExpr{Node: n, Value: "0"},
	}
	innerCond := &sem.CondExpr{
		Node: n,
		Cond: lenCond,
		Then: indexExpr,
		Else: sem.NewStructuredError(n, "subquery expression cannot have multiple columns", sem.NewThis(n, nil)),
	}
	isErrCond := &sem.CallExpr{
		Node: n,
		Tag:  "is_error",
		Args: []sem.Expr{sem.NewThis(n, nil)},
	}
	outerCond := &sem.CondExpr{
		Node: n,
		Cond: isErrCond,
		Then: sem.NewThis(n, nil),
		Else: innerCond,
	}
	return &sem.ValuesOp{
		Node:  n,
		Exprs: []sem.Expr{outerCond},
	}
}

func isCorrelated(seq sem.Seq) bool {
	if len(seq) >= 1 {
		//XXX fragile
		_, ok1 := seq[0].(*sem.FileScan)
		_, ok2 := seq[0].(*sem.PoolScan)
		return !(ok1 || ok2)
	}
	return true
}
