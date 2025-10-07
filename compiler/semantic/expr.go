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

func (t *translator) semExpr(e ast.Expr) sem.Expr {
	switch e := e.(type) {
	case *ast.Agg:
		expr := t.semExprNullable(e.Expr)
		nameLower := strings.ToLower(e.Name)
		if expr == nil && nameLower != "count" {
			t.error(e, fmt.Errorf("aggregator '%s' requires argument", e.Name))
			return badExpr()
		}
		where := t.semExprNullable(e.Where)
		return &sem.AggFunc{
			Node:     e,
			Name:     nameLower,
			Distinct: e.Distinct,
			Expr:     expr,
			Where:    where,
		}
	case *ast.ArrayExpr:
		elems := t.semArrayElems(e.Elems)
		if subquery := t.arraySubquery(elems); subquery != nil {
			return subquery
		}
		return &sem.ArrayExpr{
			Node:  e,
			Elems: elems,
		}
	case *ast.BinaryExpr:
		return t.semBinary(e)
	case *ast.Between:
		val := t.semExpr(e.Expr)
		lower := t.semExpr(e.Lower)
		upper := t.semExpr(e.Upper)
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
	case *ast.Conditional:
		cond := t.semExpr(e.Cond)
		thenExpr := t.semExpr(e.Then)
		var elseExpr sem.Expr
		if e.Else != nil {
			elseExpr = t.semExpr(e.Else)
		} else {
			elseExpr = &sem.LiteralExpr{Node: e, Value: `error("missing")`}
		}
		return &sem.CondExpr{
			Node: e,
			Cond: cond,
			Then: thenExpr,
			Else: elseExpr,
		}
	case *ast.Call:
		return t.semCall(e)
	case *ast.CallExtract:
		return t.semCallExtract(e, e.Part, e.Expr)
	case *ast.Cast:
		expr := t.semExpr(e.Expr)
		typ := t.semExpr(e.Type)
		return sem.NewCall(e, "cast", []sem.Expr{expr, typ})
	case *ast.DoubleQuote:
		return t.semDoubleQuote(e)
	case *ast.Exists:
		return t.semExists(e)
	case *ast.FString:
		return t.semFString(e)
	case *ast.FuncName:
		// We get here for &refs that are in a call expression. e.g.,
		// an arg to another function.  These are only built-ins as
		// user functions should be referenced directly as an ID.
		tag := e.Name
		if boundTag, _ := t.scope.lookupFunc(e.Name); boundTag != "" {
			tag = boundTag
		}
		return &sem.FuncRef{
			Node: e,
			Tag:  tag,
		}
	case *ast.Glob:
		return &sem.RegexpSearchExpr{
			Node:    e,
			Pattern: reglob.Reglob(e.Pattern),
			Expr:    sem.NewThis(e, nil),
		}
	case *ast.ID:
		id := t.semID(e, false)
		if t.scope.schema != nil {
			if this, ok := id.(*sem.ThisExpr); ok {
				path, err := t.scope.resolve(this.Path)
				if err != nil {
					t.error(e, err)
					return badExpr()
				}
				return sem.NewThis(e, path)
			}
		}
		return id
	case *ast.IndexExpr:
		expr := t.semExpr(e.Expr)
		index := t.semExpr(e.Index)
		// If expr is a path and index is a string, then just extend the path.
		if path := t.isIndexOfThis(expr, index); path != nil {
			return path
		}
		return &sem.IndexExpr{
			Node:  e,
			Expr:  expr,
			Index: index,
		}
	case *ast.IsNullExpr:
		expr := t.semExpr(e.Expr)
		var out sem.Expr = &sem.IsNullExpr{Node: e, Expr: expr}
		if e.Not {
			out = sem.NewUnaryExpr(e, "!", out)
		}
		return out
	case *ast.Lambda:
		tag := t.newFunc(e, "lambda", idsAsStrings(e.Params), t.semExpr(e.Expr))
		return &sem.FuncRef{
			Node: e,
			Tag:  tag,
		}
	case *ast.MapExpr:
		var entries []sem.Entry
		for _, entry := range e.Entries {
			key := t.semExpr(entry.Key)
			val := t.semExpr(entry.Value)
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
	case *ast.Subquery:
		return t.semSubquery(e, e.Array, e.Body)
	case *ast.RecordExpr:
		fields := map[string]struct{}{}
		var out []sem.RecordElem
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *ast.FieldExpr:
				if _, ok := fields[elem.Name.Text]; ok {
					t.error(elem, fmt.Errorf("record expression: %w", &super.DuplicateFieldError{Name: elem.Name.Text}))
					continue
				}
				fields[elem.Name.Text] = struct{}{}
				e := t.semExpr(elem.Value)
				out = append(out, &sem.FieldElem{
					Node:  elem,
					Name:  elem.Name.Text,
					Value: e,
				})
			case *ast.ID:
				if _, ok := fields[elem.Name]; ok {
					t.error(elem, fmt.Errorf("record expression: %w", &super.DuplicateFieldError{Name: elem.Name}))
					continue
				}
				fields[elem.Name] = struct{}{}
				// Call semExpr even though we know this is an ID so
				// SQL-context scope mappings are carried out.
				v := t.semExpr(elem)
				out = append(out, &sem.FieldElem{
					Node:  elem,
					Name:  elem.Name,
					Value: v,
				})
			case *ast.Spread:
				e := t.semExpr(elem.Expr)
				out = append(out, &sem.SpreadElem{
					Node: elem,
					Expr: e,
				})
			default:
				e := t.semExpr(elem)
				name := deriveNameFromExpr(e, elem)
				if _, ok := fields[name]; ok {
					t.error(elem, fmt.Errorf("record expression: %w", &super.DuplicateFieldError{Name: name}))
					continue
				}
				fields[name] = struct{}{}
				out = append(out, &sem.FieldElem{
					Name:  name,
					Value: e,
				})
			}
		}
		return &sem.RecordExpr{
			Node:  e,
			Elems: out,
		}
	case *ast.Regexp:
		return &sem.RegexpSearchExpr{
			Node:    e,
			Pattern: e.Pattern,
			Expr:    sem.NewThis(e, nil),
		}
	case *ast.SetExpr:
		elems := t.semArrayElems(e.Elems)
		return &sem.SetExpr{
			Node:  e,
			Elems: elems,
		}
	case *ast.SliceExpr:
		expr := t.semExpr(e.Expr)
		// XXX Literal indices should be type checked as int.
		from := t.semExprNullable(e.From)
		to := t.semExprNullable(e.To)
		return &sem.SliceExpr{
			Node: e,
			Expr: expr,
			From: from,
			To:   to,
		}
	case *ast.SQLCast:
		expr := t.semExpr(e.Expr)
		if _, ok := e.Type.(*ast.DateTypeHack); ok {
			// cast to time then bucket by 1d as a workaround for not currently
			// supporting a "date" type.
			cast := sem.NewCall(e, "cast", []sem.Expr{expr, &sem.LiteralExpr{Node: e, Value: "<time>"}})
			return sem.NewCall(e, "bucket", []sem.Expr{cast, &sem.LiteralExpr{Node: e, Value: "1d"}})
		}
		typ := t.semExpr(&ast.TypeValue{
			Kind:  "TypeValue",
			Value: e.Type,
		})
		return sem.NewCall(e, "cast", []sem.Expr{expr, typ})
	case *ast.SQLSubstring:
		expr := t.semExpr(e.Expr)
		if e.From == nil && e.For == nil {
			t.error(e, errors.New("FROM or FOR must be set"))
			return badExpr()
		}
		// XXX type checker should remove this check when it finds it redundant
		is := sem.NewCall(e, "is", []sem.Expr{expr, &sem.LiteralExpr{Node: e.Expr, Value: "<string>"}})
		slice := &sem.SliceExpr{
			Node: e,
			Expr: expr,
			From: t.semExprNullable(e.From),
		}
		if e.For != nil {
			to := t.semExpr(e.For)
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
	case *ast.SQLTimeValue:
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
	case *ast.Term:
		var val string
		switch term := e.Value.(type) {
		case *ast.Primitive:
			v, err := sup.ParsePrimitive(term.Type, term.Text)
			if err != nil {
				t.error(e, err)
				return badExpr()
			}
			val = sup.FormatValue(v)
		case *ast.DoubleQuote:
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
	case *ast.TupleExpr:
		elems := make([]sem.RecordElem, 0, len(e.Elems))
		for colno, elem := range e.Elems {
			e := t.semExpr(elem)
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
		operand := t.semExpr(e.Operand)
		if e.Op == "+" {
			return operand
		}
		return sem.NewUnaryExpr(e, e.Op, operand)
	case nil:
		panic("semantic analysis: illegal null value encountered in AST")
	}
	panic(e)
}

func (t *translator) semID(id *ast.ID, lval bool) sem.Expr {
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
		if _, ok := entry.ref.(*sem.FuncDef); ok && !lval {
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

func (t *translator) semDoubleQuote(d *ast.DoubleQuote) sem.Expr {
	// Check if there's a SQL scope and treat a double-quoted string
	// as an identifier.  XXX we'll need to do something a bit more
	// sophisticated to handle pipes inside SQL subqueries.
	if t.scope.schema != nil {
		return t.semExpr(&ast.ID{Kind: "ID", Name: d.Text, Loc: d.Loc})
	}
	return t.semExpr(&ast.Primitive{
		Kind: "Primitive",
		Type: "string",
		Text: d.Text,
		Loc:  d.Loc,
	})
}

func (t *translator) semExists(e *ast.Exists) sem.Expr {
	q := t.semSubquery(e, true, e.Body)
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

func (t *translator) semRegexp(b *ast.BinaryExpr) sem.Expr {
	if b.Op != "~" {
		return nil
	}
	s, ok := t.mustEvalString(t.semExpr(b.RHS))
	if !ok {
		t.error(b, errors.New(`right-hand side of ~ expression must be a string literal`))
		return badExpr()
	}
	if _, err := expr.CompileRegexp(s); err != nil {
		t.error(b.RHS, err)
		return badExpr()
	}
	e := t.semExpr(b.LHS)
	return &sem.RegexpMatchExpr{
		Node:    b,
		Pattern: s,
		Expr:    e,
	}
}

func (t *translator) semBinary(e *ast.BinaryExpr) sem.Expr {
	if path, bad := t.semDotted(e, false); path != nil {
		if t.scope.schema != nil {
			path, err := t.scope.resolve(path)
			if err != nil {
				t.error(e, err)
				return badExpr()
			}
			return sem.NewThis(e, path)
		}
		return sem.NewThis(e, path)
	} else if bad != nil {
		return bad
	}
	if e := t.semRegexp(e); e != nil {
		return e
	}
	op := strings.ToLower(e.Op)
	if op == "." {
		lhs := t.semExpr(e.LHS)
		id, ok := e.RHS.(*ast.ID)
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
	lhs := t.semExpr(e.LHS)
	rhs := t.semExpr(e.RHS)
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

func (t *translator) semExprNullable(e ast.Expr) sem.Expr {
	if e == nil {
		return nil
	}
	return t.semExpr(e)
}

func (t *translator) semDotted(e *ast.BinaryExpr, lval bool) ([]string, sem.Expr) {
	if e.Op != "." {
		return nil, nil
	}
	rhs, ok := e.RHS.(*ast.ID)
	if !ok {
		return nil, nil
	}
	switch lhs := e.LHS.(type) {
	case *ast.ID:
		switch e := t.semID(lhs, lval).(type) {
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
	e := t.semExpr(c.Expr)
	out := t.semExprNullable(c.Else)
	for i := len(c.Whens) - 1; i >= 0; i-- {
		when := c.Whens[i]
		out = &sem.CondExpr{
			Node: c,
			Cond: sem.NewBinaryExpr(c, "==", e, t.semExpr(when.Cond)),
			Then: t.semExpr(when.Then),
			Else: out,
		}
	}
	return out
}

func (t *translator) semCall(call *ast.Call) sem.Expr {
	if e := t.maybeConvertAgg(call); e != nil {
		return e
	}
	if call.Where != nil {
		t.error(call, errors.New("'where' clause on non-aggregation function"))
		return badExpr()
	}
	args := t.semExprs(call.Args)
	switch f := call.Func.(type) {
	case *ast.FuncName:
		return t.semCallByName(call, f.Name, args)
	case *ast.Lambda:
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

func (t *translator) semCallLambda(lambda *ast.Lambda, args []sem.Expr) sem.Expr {
	tag := t.newFunc(lambda, "lambda", idsAsStrings(lambda.Params), t.semExpr(lambda.Expr))
	return sem.NewCall(lambda, tag, args)
}

func (t *translator) semCallByName(call *ast.Call, name string, args []sem.Expr) sem.Expr {
	if subquery := t.maybeSubqueryCall(call, name); subquery != nil {
		return subquery
	}
	// Check if the name resolves to a symbol in scope.
	if entry := t.scope.lookupEntry(name); entry != nil {
		switch ref := entry.ref.(type) {
		case param:
			// Called name is a parameter inside of a function.   We create a dummy
			// CallParam that will be converted to a direct call to the passed-in
			// function (we don't know it yet and there may be multiple variations
			// that all land at this call site) in the next pass of semantic analysis.
			return &sem.CallParam{
				Node:  call,
				Param: name,
				Args:  args,
			}
		case *opDecl:
			t.error(call, fmt.Errorf("cannot call user operator %q in an expression (consider subquery syntax)", name))
			return badExpr()
		case *sem.FuncRef:
			return sem.NewCall(call, ref.Tag, args)
		case *sem.FuncDef:
			return sem.NewCall(call, ref.Tag, args)
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
	// Call could be to a user func. Check if we have a matching func in scope.
	// When the name is a formal argument, the bindings will have been put
	// in scope and will point to the right entity (a builtin function name or a FuncDef).
	tag, _ := t.scope.lookupFunc(name)
	nargs := len(args)
	// udf should be checked first since a udf can override builtin functions.
	if f := t.funcs[tag]; f != nil {
		if len(f.Params) != nargs {
			t.error(call, fmt.Errorf("call expects %d argument(s)", len(f.Params)))
			return badExpr()
		}
		return sem.NewCall(call, f.Tag, args)
	}
	if tag != "" {
		name = tag
	}
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

func (t *translator) maybeSubqueryCall(call *ast.Call, name string) *sem.SubqueryExpr {
	decl, _ := t.scope.lookupOp(name)
	if decl == nil || decl.bad {
		return nil
	}
	userOp := t.semUserOp(call.Loc, decl, call.Args)
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

func (t *translator) semMapCall(call *ast.Call, args []sem.Expr) sem.Expr {
	if len(args) != 2 {
		t.error(call, errors.New("map requires two arguments"))
		return badExpr()
	}
	f, ok := args[1].(*sem.FuncRef)
	if !ok {
		t.error(call, errors.New("second argument to map must be a function"))
		return badExpr()
	}
	e := &sem.MapCallExpr{
		Node:   call,
		Expr:   args[0],
		Lambda: sem.NewCall(call.Args[1], f.Tag, []sem.Expr{sem.NewThis(call.Args[1], nil)}),
	}
	return e
}

func (t *translator) semCallExtract(e, partExpr, argExpr ast.Expr) sem.Expr {
	var partstr string
	switch p := partExpr.(type) {
	case *ast.ID:
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
			t.semExpr(argExpr),
		},
	)
}

func (t *translator) semExprs(in []ast.Expr) []sem.Expr {
	exprs := make([]sem.Expr, 0, len(in))
	for _, e := range in {
		exprs = append(exprs, t.semExpr(e))
	}
	return exprs
}

func (t *translator) semAssignments(assignments []ast.Assignment) []sem.Assignment {
	out := make([]sem.Assignment, 0, len(assignments))
	for _, e := range assignments {
		out = append(out, t.semAssignment(&e))
	}
	return out
}

func (t *translator) semAssignment(assign *ast.Assignment) sem.Assignment {
	rhs := t.semExpr(assign.RHS)
	var lhs sem.Expr
	if assign.LHS == nil {
		lhs = sem.NewThis(assign.RHS, []string{deriveNameFromExpr(rhs, assign.RHS)})
	} else {
		lhs = t.semLval(assign.LHS)
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

func (t *translator) semLval(e ast.Expr) sem.Expr {
	if id, ok := e.(*ast.ID); ok {
		return t.semID(id, true)
	}
	return t.semExpr(e)
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

func deriveNameFromExpr(e sem.Expr, a ast.Expr) string {
	switch a := a.(type) {
	case *ast.Agg:
		return a.Name
	case *ast.Call:
		var name string
		if f, ok := a.Func.(*ast.FuncName); ok {
			name = f.Name
		}
		if strings.ToLower(name) == "quiet" {
			if e, ok := e.(*sem.CallExpr); ok && len(e.Args) > 0 {
				if this, ok := e.Args[0].(*sem.ThisExpr); ok {
					return fieldOfThis(this)
				}
			}
		}
		return name
	}
	if e, ok := e.(*sem.ThisExpr); ok {
		return fieldOfThis(e)
	}
	return sfmt.ASTExpr(a)
}

func fieldOfThis(this *sem.ThisExpr) string {
	if len(this.Path) == 0 {
		return "that"
	}
	return this.Path[len(this.Path)-1]
}

func (t *translator) semFields(exprs []ast.Expr) []sem.Expr {
	fields := make([]sem.Expr, 0, len(exprs))
	for _, e := range exprs {
		fields = append(fields, t.semField(e))
	}
	return fields
}

// semField analyzes the expression f and makes sure that it's
// a field reference returning an error if not.
func (t *translator) semField(f ast.Expr) sem.Expr {
	switch e := t.semExpr(f).(type) {
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

func (t *translator) maybeConvertAgg(call *ast.Call) sem.Expr {
	name, ok := call.Func.(*ast.FuncName)
	if !ok {
		return nil
	}
	nameLower := strings.ToLower(name.Name)
	if _, err := agg.NewPattern(nameLower, false, true); err != nil {
		return nil
	}
	var e sem.Expr
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
	if len(call.Args) == 1 {
		e = t.semExpr(call.Args[0])
	}
	return &sem.AggFunc{
		Node:  call,
		Name:  nameLower,
		Expr:  e,
		Where: t.semExprNullable(call.Where),
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

func (t *translator) semArrayElems(elems []ast.VectorElem) []sem.ArrayElem {
	var out []sem.ArrayElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *ast.Spread:
			out = append(out, &sem.SpreadElem{Node: elem, Expr: t.semExpr(elem.Expr)})
		case *ast.VectorValue:
			out = append(out, &sem.ExprElem{Node: elem, Expr: t.semExpr(elem.Expr)})
		default:
			panic(elem)
		}
	}
	return out
}

func (t *translator) semFString(f *ast.FString) sem.Expr {
	if len(f.Elems) == 0 {
		return &sem.LiteralExpr{Node: f, Value: `""`}
	}
	var out sem.Expr
	for _, elem := range f.Elems {
		var e sem.Expr
		switch elem := elem.(type) {
		case *ast.FStringExpr:
			e = t.semExpr(elem.Expr)
			e = sem.NewCall(f,
				"cast",
				[]sem.Expr{e, &sem.LiteralExpr{Value: "<string>"}})
		case *ast.FStringText:
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

func (t *translator) semSubquery(astExpr ast.Expr, array bool, body ast.Seq) *sem.SubqueryExpr {
	seq := t.semSeq(body)
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
	if isSQLOp(body[0]) { //XXX this should check scope not isSQLOp?
		// SQL expects a record with a single column result so fetch the first
		// value.
		// XXX this should be a structured error... or just allow it
		// SQL expects a record with a single column result so fetch the first
		// value.  Or we should be descoping.
		// XXX subquery runtime should return the error and user needs to wrap []
		// if they expect multiple values
		e.Body.Append(&sem.ValuesOp{
			Node: astExpr,
			Exprs: []sem.Expr{
				&sem.IndexExpr{
					Node:  astExpr,
					Expr:  sem.NewThis(astExpr, nil),
					Index: &sem.LiteralExpr{Node: astExpr, Value: "1"},
				}},
		})
	}
	return e
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
