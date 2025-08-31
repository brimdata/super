package semantic

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/araddon/dateparse"
	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/rungen"
	"github.com/brimdata/super/compiler/sfmt"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/pkg/reglob"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/sup"
	"github.com/shellyln/go-sql-like-expr/likeexpr"
)

func (a *analyzer) semExpr(e ast.Expr) dag.Expr {
	switch e := e.(type) {
	case *ast.Agg:
		expr := a.semExprNullable(e.Expr)
		nameLower := strings.ToLower(e.Name)
		if expr == nil && nameLower != "count" {
			a.error(e, fmt.Errorf("aggregator '%s' requires argument", e.Name))
			return badExpr()
		}
		where := a.semExprNullable(e.Where)
		return &dag.Agg{
			Kind:     "Agg",
			Name:     nameLower,
			Distinct: e.Distinct,
			Expr:     expr,
			Where:    where,
		}
	case *ast.ArrayExpr:
		elems := a.semVectorElems(e.Elems)
		if subquery := a.arraySubquery(elems); subquery != nil {
			return subquery
		}
		return &dag.ArrayExpr{
			Kind:  "ArrayExpr",
			Elems: elems,
		}
	case *ast.BinaryExpr:
		return a.semBinary(e)
	case *ast.Between:
		val := a.semExpr(e.Expr)
		lower := a.semExpr(e.Lower)
		upper := a.semExpr(e.Upper)
		expr := dag.NewBinaryExpr("and",
			dag.NewBinaryExpr(">=", val, lower),
			dag.NewBinaryExpr("<=", val, upper))
		if e.Not {
			return dag.NewUnaryExpr("!", expr)
		}
		return expr
	case *ast.CaseExpr:
		return a.semCaseExpr(e)
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
	case *ast.CallExtract:
		return a.semCallExtract(e.Part, e.Expr)
	case *ast.Cast:
		expr := a.semExpr(e.Expr)
		typ := a.semExpr(e.Type)
		return &dag.Call{
			Kind: "Call",
			Func: &dag.FuncName{Kind: "FuncName", Name: "cast"},
			Args: []dag.Expr{expr, typ},
		}
	case *ast.DoubleQuote:
		return a.semDoubleQuote(e)
	case *ast.Exists:
		return a.semExists(e)
	case *ast.FString:
		return a.semFString(e)
	case *ast.Glob:
		return &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: reglob.Reglob(e.Pattern),
			Expr:    pathOf("this"),
		}
	case *ast.ID:
		id := a.semID(e)
		if a.scope.schema != nil {
			if this, ok := id.(*dag.This); ok {
				out, err := a.scope.resolve(this.Path)
				if err != nil {
					a.error(e, err)
					return badExpr()
				}
				return out
			}
		}
		return id
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
			out = dag.NewUnaryExpr("!", out)
		}
		return out
	case *ast.MapCall:
		return a.semMapCall(e)
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
	case *ast.Primitive:
		val, err := sup.ParsePrimitive(e.Type, e.Text)
		if err != nil {
			a.error(e, err)
			return badExpr()
		}
		return &dag.Literal{
			Kind:  "Literal",
			Value: sup.FormatValue(val),
		}
	case *ast.Subquery:
		return a.semSubquery(e.Body)
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
	case *ast.Regexp:
		return &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: e.Pattern,
			Expr:    pathOf("this"),
		}
	case *ast.SetExpr:
		elems := a.semVectorElems(e.Elems)
		return &dag.SetExpr{
			Kind:  "SetExpr",
			Elems: elems,
		}
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
	case *ast.SQLCast:
		expr := a.semExpr(e.Expr)
		if _, ok := e.Type.(*ast.DateTypeHack); ok {
			// cast to time then bucket by 1d as a workaround for not currently
			// supporting a "date" type.
			cast := dag.NewCallByName(
				"cast",
				[]dag.Expr{expr, &dag.Literal{Kind: "Literal", Value: "<time>"}},
			)
			return dag.NewCallByName(
				"bucket",
				[]dag.Expr{cast, &dag.Literal{Kind: "Literal", Value: "1d"}},
			)
		}
		typ := a.semExpr(&ast.TypeValue{
			Kind:  "TypeValue",
			Value: e.Type,
		})
		return dag.NewCallByName("cast", []dag.Expr{expr, typ})
	case *ast.SQLSubstring:
		expr := a.semExpr(e.Expr)
		if e.From == nil && e.For == nil {
			a.error(e, errors.New("FROM or FOR must be set"))
			return badExpr()
		}
		is := dag.NewCallByName(
			"is",
			[]dag.Expr{expr, &dag.Literal{Kind: "Literal", Value: "<string>"}},
		)
		slice := &dag.SliceExpr{Kind: "SliceExpr", Expr: expr, From: a.semExprNullable(e.From)}
		if e.For != nil {
			to := a.semExpr(e.For)
			if slice.From != nil {
				slice.To = dag.NewBinaryExpr("+", slice.From, to)
			} else {
				slice.To = dag.NewBinaryExpr("+", to, &dag.Literal{Kind: "Literal", Value: "1"})
			}
		}
		errRec := &dag.RecordExpr{
			Kind: "RecordExpr",
			Elems: []dag.RecordElem{
				&dag.Field{Kind: "Field", Name: "message", Value: &dag.Literal{Kind: "Literal", Value: `"SUBSTRING: string value required"`}},
				&dag.Field{Kind: "Field", Name: "value", Value: expr},
			},
		}
		return &dag.Conditional{
			Kind: "Conditional",
			Cond: is,
			Then: slice,
			Else: dag.NewCallByName("error", []dag.Expr{errRec}),
		}
	case *ast.SQLTimeValue:
		if e.Value.Type != "string" {
			a.error(e.Value, errors.New("value must be a string literal"))
			return badExpr()
		}
		t, err := dateparse.ParseAny(e.Value.Text)
		if err != nil {
			a.error(e.Value, err)
			return badExpr()
		}
		ts := nano.TimeToTs(t)
		if e.Type == "date" {
			ts = ts.Trunc(nano.Day)
		}
		return &dag.Literal{Kind: "Literal", Value: sup.FormatValue(super.NewTime(ts))}
	case *ast.Term:
		var val string
		switch t := e.Value.(type) {
		case *ast.Primitive:
			v, err := sup.ParsePrimitive(t.Type, t.Text)
			if err != nil {
				a.error(e, err)
				return badExpr()
			}
			val = sup.FormatValue(v)
		case *ast.DoubleQuote:
			v, err := sup.ParsePrimitive("string", t.Text)
			if err != nil {
				a.error(e, err)
				return badExpr()
			}
			val = sup.FormatValue(v)
		case *ast.TypeValue:
			tv, err := a.semType(t.Value)
			if err != nil {
				a.error(e, err)
				return badExpr()
			}
			val = "<" + tv + ">"
		default:
			panic(fmt.Errorf("unexpected term value: %s (%T)", e.Kind, e))
		}
		return &dag.Search{
			Kind:  "Search",
			Text:  e.Text,
			Value: val,
			Expr:  pathOf("this"),
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
	case *ast.UnaryExpr:
		operand := a.semExpr(e.Operand)
		if e.Op == "+" {
			return operand
		}
		return dag.NewUnaryExpr(e.Op, operand)
	case nil:
		panic("semantic analysis: illegal null value encountered in AST")
	}
	panic(errors.New("invalid expression type"))
}

func (a *analyzer) semID(id *ast.ID) dag.Expr {
	// We use static scoping here to see if an identifier is
	// a "var" reference to the name or a field access
	// and transform the AST node appropriately.  The resulting DAG
	// doesn't have Identifiers as they are resolved here
	// one way or the other.
	if subquery := a.maybeSubqueryCallByID(id); subquery != nil {
		return subquery
	}
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

func (a *analyzer) semDoubleQuote(d *ast.DoubleQuote) dag.Expr {
	// Check if there's a SQL scope and treat a double-quoted string
	// as an identifier.  XXX we'll need to do something a bit more
	// sophisticated to handle pipes inside SQL subqueries.
	if a.scope.schema != nil {
		return a.semExpr(&ast.ID{Kind: "ID", Name: d.Text, Loc: d.Loc})
	}
	return a.semExpr(&ast.Primitive{
		Kind: "Primitive",
		Type: "string",
		Text: d.Text,
		Loc:  d.Loc,
	})
}

func (a *analyzer) semExists(e *ast.Exists) dag.Expr {
	q := a.semSubquery(e.Body)
	// Append collect(this) to ensure array of results is returned.
	q.Body = appendCollect(append(q.Body, &dag.Head{Kind: "Head", Count: 1}))
	return dag.NewBinaryExpr(">",
		dag.NewCallByName("len", []dag.Expr{q}),
		&dag.Literal{Kind: "Literal", Value: "0"})
}

func appendCollect(body dag.Seq) dag.Seq {
	return append(body,
		&dag.Aggregate{
			Kind: "Aggregate",
			Aggs: []dag.Assignment{{
				Kind: "Assignment",
				LHS:  pathOf("collect"),
				RHS:  &dag.Agg{Kind: "Agg", Name: "collect", Expr: dag.NewThis(nil)},
			}},
		},
		&dag.Values{Kind: "Values", Exprs: []dag.Expr{pathOf("collect")}},
	)
}

func semDynamicType(tv ast.Type) *dag.Call {
	if typeName, ok := tv.(*ast.TypeName); ok {
		return dynamicTypeName(typeName.Name)
	}
	return nil
}

func dynamicTypeName(name string) *dag.Call {
	return dag.NewCallByName(
		"typename",
		[]dag.Expr{
			// SUP string literal of type name
			&dag.Literal{
				Kind:  "Literal",
				Value: `"` + name + `"`,
			},
		},
	)
}

func (a *analyzer) semRegexp(b *ast.BinaryExpr) dag.Expr {
	if b.Op != "~" {
		return nil
	}
	s, ok := isStringConst(a.sctx, a.semExpr(b.RHS))
	if !ok {
		a.error(b, errors.New(`right-hand side of ~ expression must be a string literal`))
		return badExpr()
	}
	if _, err := expr.CompileRegexp(s); err != nil {
		a.error(b.RHS, err)
		return badExpr()
	}
	e := a.semExpr(b.LHS)
	return &dag.RegexpMatch{
		Kind:    "RegexpMatch",
		Pattern: s,
		Expr:    e,
	}
}

func (a *analyzer) semBinary(e *ast.BinaryExpr) dag.Expr {
	if path, bad := a.semDotted(e); path != nil {
		if a.scope.schema != nil {
			out, err := a.scope.resolve(path)
			if err != nil {
				a.error(e, err)
				return badExpr()
			}
			return out
		}
		return dag.NewThis(path)
	} else if bad != nil {
		return bad
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
	if op == "like" || op == "not like" {
		s, ok := isStringConst(a.sctx, rhs)
		if !ok {
			a.error(e.RHS, errors.New("non-constant pattern for LIKE not supported"))
			return badExpr()
		}
		pattern := likeexpr.ToRegexp(s, '\\', false)
		expr := &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: "(?s)" + pattern,
			Expr:    lhs,
		}
		if op == "not like" {
			return dag.NewUnaryExpr("!", expr)
		}
		return expr
	}
	if op == "in" || op == "not in" {
		if q, ok := rhs.(*dag.Subquery); ok {
			q.Body = appendCollect(q.Body)
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
		return dag.NewUnaryExpr("!", dag.NewBinaryExpr("in", lhs, rhs))
	case "::":
		return dag.NewCallByName("cast", []dag.Expr{lhs, rhs})
	}
	return dag.NewBinaryExpr(op, lhs, rhs)
}

func (a *analyzer) isIndexOfThis(lhs, rhs dag.Expr) *dag.This {
	if this, ok := lhs.(*dag.This); ok {
		if s, ok := isStringConst(a.sctx, rhs); ok {
			this.Path = append(this.Path, s)
			return this
		}
	}
	return nil
}

func isStringConst(sctx *super.Context, e dag.Expr) (field string, ok bool) {
	val, err := rungen.EvalAtCompileTime(sctx, e)
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

func (a *analyzer) semDotted(e *ast.BinaryExpr) ([]string, dag.Expr) {
	if e.Op != "." {
		return nil, nil
	}
	rhs, ok := e.RHS.(*ast.ID)
	if !ok {
		return nil, nil
	}
	switch lhs := e.LHS.(type) {
	case *ast.ID:
		switch e := a.semID(lhs).(type) {
		case *dag.This:
			return append(slices.Clone(e.Path), rhs.Name), nil
		case *dag.BadExpr:
			return nil, e
		default:
			return nil, nil
		}
	case *ast.BinaryExpr:
		this, bad := a.semDotted(lhs)
		if this == nil {
			return nil, bad
		}
		return append(this, rhs.Name), nil
	}
	return nil, nil
}

func (a *analyzer) semCaseExpr(c *ast.CaseExpr) dag.Expr {
	e := a.semExpr(c.Expr)
	out := a.semExprNullable(c.Else)
	for i := len(c.Whens) - 1; i >= 0; i-- {
		when := c.Whens[i]
		out = &dag.Conditional{
			Kind: "Conditional",
			Cond: dag.NewBinaryExpr("==", dag.CopyExpr(e), a.semExpr(when.Cond)),
			Then: a.semExpr(when.Then),
			Else: out,
		}
	}
	return out
}

func (a *analyzer) semCall(call *ast.Call) dag.Expr {
	if e := a.maybeConvertAgg(call); e != nil {
		return e
	}
	if call.Where != nil {
		a.error(call, errors.New("'where' clause on non-aggregation function"))
		return badExpr()
	}
	args := a.semExprs(call.Args)
	switch f := call.Func.(type) {
	case *ast.FuncName:
		return a.semCallByName(call, f.Name, args)
	case *ast.Lambda:
		return a.semCallLambda(f, args)
	default:
		panic(f)
	}
}

func (a *analyzer) semCallLambda(lambda *ast.Lambda, args []dag.Expr) dag.Expr {
	params := make([]string, 0, len(lambda.Params))
	for _, id := range lambda.Params {
		params = append(params, id.Name)
	}
	return &dag.Call{
		Kind: "Call",
		Func: &dag.Lambda{
			Kind:   "Lambda",
			Params: params,
			Expr:   a.semExpr(lambda.Expr),
		},
		Args: args,
	}
}

func (a *analyzer) semCallByName(call *ast.Call, name string, args []dag.Expr) dag.Expr {
	if subquery := a.maybeSubqueryCall(call, name); subquery != nil {
		return subquery
	}
	// Call could be to a user defined func. Check if we have a matching func in
	// scope.  The name can be a formal argument of a user op so we look it up and
	// see if it points to something else.
	f, lambda, err := a.scope.LookupFunc(name)
	if err != nil {
		a.error(call, err)
		return badExpr()
	}
	if lambda != nil {
		// This can happen when a lambda is passed in and the arg
		// is referred to with a name, e.g,
		// op foo f : (values f(foo)) call foo lambda x:x+1
		return &dag.Call{
			Kind: "Call",
			Func: &dag.Lambda{
				Kind:   "Lambda",
				Params: lambda.Params,
				Expr:   lambda.Expr,
			},
			Args: args,
		}
	}
	nargs := len(args)
	nameLower := strings.ToLower(name)
	switch {
	// udf should be checked first since a udf can override builtin functions.
	case f != nil:
		if len(f.Lambda.Params) != nargs {
			a.error(call, fmt.Errorf("call expects %d argument(s)", len(f.Lambda.Params)))
			return badExpr()
		}
		return dag.NewCallByName(f.Name, args)
	case expr.NewShaperTransform(nameLower) != 0:
		if err := function.CheckArgCount(nargs, 2, 2); err != nil {
			a.error(call, err)
			return badExpr()
		}
	case nameLower == "grep":
		if err := function.CheckArgCount(nargs, 2, 2); err != nil {
			a.error(call, err)
			return badExpr()
		}
		pattern, ok := isStringConst(a.sctx, args[0])
		if !ok {
			return dag.NewCallByName("grep", args)
		}
		re, err := expr.CompileRegexp(pattern)
		if err != nil {
			a.error(call.Args[0], err)
			return badExpr()
		}
		if s, ok := re.LiteralPrefix(); ok {
			return &dag.Search{
				Kind:  "Search",
				Text:  s,
				Value: sup.QuotedString(s),
				Expr:  args[1],
			}
		}
		return &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: pattern,
			Expr:    args[1],
		}
	case nameLower == "position" && nargs == 1:
		b, ok := args[0].(*dag.BinaryExpr)
		if ok && strings.ToLower(b.Op) == "in" {
			// Support for SQL style position function call.
			args = []dag.Expr{b.RHS, b.LHS}
			nargs = 2
		}
		fallthrough
	default:
		if _, err = function.New(a.sctx, nameLower, nargs); err != nil {
			a.error(call, err)
			return badExpr()
		}
	}
	return dag.NewCallByName(nameLower, args)
}

func (a *analyzer) maybeSubqueryCallByID(id *ast.ID) *dag.Subquery {
	call := &ast.Call{
		Kind: "Call",
		Func: &ast.FuncName{Kind: "FuncName", Name: id.Name},
		Loc:  id.Loc,
	}
	return a.maybeSubqueryCall(call, id.Name)
}

func (a *analyzer) maybeSubqueryCall(call *ast.Call, name string) *dag.Subquery {
	decl, _ := a.scope.lookupOp(name)
	if decl == nil || decl.bad {
		return nil
	}
	var args []ast.FuncOrExpr
	for _, arg := range call.Args {
		args = append(args, arg)
	}
	userOp := a.semUserOp(call.Loc, decl, args)
	// When a user op is encountered inside an expression, we turn it into a
	// subquery operating on a single-shot "this" value unless it's uncorrelated
	// (i.e., starts with a from), in which case we put the uncorrelated body
	// in the subquery without the correlating values logic.
	correlated := isCorrelated(userOp)
	if correlated {
		valuesOp := &dag.Values{
			Kind:  "Values",
			Exprs: []dag.Expr{dag.NewThis(nil)},
		}
		userOp.Prepend(valuesOp)
	}
	return &dag.Subquery{
		Kind:       "Subquery",
		Correlated: correlated,
		Body:       userOp,
	}
}

func (a *analyzer) semMapCall(call *ast.MapCall) dag.Expr {
	var lambda *dag.Call
	this := []dag.Expr{dag.NewThis(nil)}
	switch f := call.Func.(type) {
	case *ast.FuncName:
		if _, _, err := a.scope.LookupFunc(f.Name); err != nil {
			a.error(f, err)
			return badExpr()
		}
		lambda = dag.NewCallByName(f.Name, this)
	case *ast.Lambda:
		lambda = &dag.Call{
			Kind: "Call",
			Func: &dag.Lambda{
				Kind:   "Lambda",
				Params: idsAsStrings(f.Params),
				Expr:   a.semExpr(f.Expr),
			},
			Args: this,
		}
	default:
		panic("semMapCall")
	}
	return &dag.MapCall{
		Kind:   "MapCall",
		Expr:   a.semExpr(call.Expr),
		Lambda: lambda,
	}
}

func (a *analyzer) semCallExtract(partExpr, argExpr ast.Expr) dag.Expr {
	var partstr string
	switch p := partExpr.(type) {
	case *ast.ID:
		partstr = p.Name
	case *ast.Primitive:
		if p.Type != "string" {
			a.error(partExpr, fmt.Errorf("part must be an identifier or string"))
			return badExpr()
		} else {
			partstr = p.Text
		}
	default:
		a.error(partExpr, fmt.Errorf("part must be an identifier or string"))
		return badExpr()
	}
	return dag.NewCallByName(
		"date_part",
		[]dag.Expr{
			&dag.Literal{Kind: "Literal", Value: sup.QuotedString(strings.ToLower(partstr))},
			a.semExpr(argExpr),
		},
	)
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
		lhs = dag.NewThis(deriveNameFromExpr(rhs, assign.RHS))
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

func deriveNameFromExpr(e dag.Expr, a ast.Expr) []string {
	switch e := e.(type) {
	case *dag.Agg:
		return []string{e.Name}
	case *dag.Call:
		name := e.Name()
		switch strings.ToLower(name) {
		case "quiet":
			if len(e.Args) > 0 {
				if this, ok := e.Args[0].(*dag.This); ok {
					return this.Path
				}
			}
		}
		return []string{name}
	case *dag.This:
		return e.Path
	default:
		return []string{sfmt.ASTExpr(a)}
	}
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
	switch e := a.semExpr(f).(type) {
	case *dag.This:
		if len(e.Path) == 0 {
			a.error(f, errors.New("cannot use 'this' as a field reference"))
			return badExpr()
		}
		return e
	case *dag.BadExpr:
		return e
	default:
		a.error(f, errors.New("invalid expression used as a field"))
		return badExpr()
	}
}

func (a *analyzer) maybeConvertAgg(call *ast.Call) dag.Expr {
	name, ok := call.Func.(*ast.FuncName)
	if !ok {
		return nil
	}
	nameLower := strings.ToLower(name.Name)
	if _, err := agg.NewPattern(nameLower, false, true); err != nil {
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
	return dag.NewThis(path)
}

func (a *analyzer) semType(typ ast.Type) (string, error) {
	ztype, err := sup.TranslateType(a.sctx, typ)
	if err != nil {
		return "", err
	}
	return sup.FormatType(ztype), nil
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
			e = dag.NewCallByName(
				"cast",
				[]dag.Expr{e, &dag.Literal{Kind: "Literal", Value: "<string>"}},
			)
		case *ast.FStringText:
			e = &dag.Literal{Kind: "Literal", Value: sup.QuotedString(elem.Text)}
		default:
			panic(fmt.Errorf("internal error: unsupported f-string elem %T", elem))
		}
		if out == nil {
			out = e
			continue
		}
		out = dag.NewBinaryExpr("+", out, e)
	}
	return out
}

func (a *analyzer) arraySubquery(elems []dag.VectorElem) *dag.Subquery {
	if len(elems) != 1 {
		return nil
	}
	elem, ok := elems[0].(*dag.VectorValue)
	if !ok {
		return nil
	}
	subquery, ok := elem.Expr.(*dag.Subquery)
	if !ok {
		return nil
	}
	subquery.Body = collectThis(subquery.Body)
	return subquery
}

func collectThis(seq dag.Seq) dag.Seq {
	collect := dag.Assignment{
		Kind: "Assignment",
		LHS:  pathOf("collect"),
		RHS:  &dag.Agg{Kind: "Agg", Name: "collect", Expr: dag.NewThis(nil)},
	}
	aggOp := &dag.Aggregate{
		Kind: "Aggregate",
		Aggs: []dag.Assignment{collect},
	}
	emitOp := &dag.Values{
		Kind:  "Values",
		Exprs: []dag.Expr{pathOf("collect")},
	}
	seq = append(seq, aggOp)
	return append(seq, emitOp)
}

func (a *analyzer) semSubquery(b ast.Seq) *dag.Subquery {
	body := a.semSeq(b)
	correlated := isCorrelated(body)
	e := &dag.Subquery{
		Kind:       "Subquery",
		Correlated: correlated,
		Body:       body,
	}
	// Add a nullscan only for uncorrelated queries that don't have a source.
	if !correlated && !HasSource(e.Body) {
		e.Body.Prepend(&dag.NullScan{Kind: "NullScan"})
	}
	if isSQLOp(b[0]) { //XXX this should check scope not isSQLOp?
		// SQL expects a record with a single column result so fetch the first
		// value.
		// XXX this should be a structured error... or just allow it
		// SQL expects a record with a single column result so fetch the first
		// value.  Or we should be descoping.
		e.Body.Append(&dag.Values{
			Kind: "Values",
			Exprs: []dag.Expr{
				&dag.IndexExpr{
					Kind:  "IndexExpr",
					Expr:  dag.NewThis(nil),
					Index: &dag.Literal{Kind: "Literal", Value: "1"},
				}},
		})
	}
	return e
}

func isCorrelated(seq dag.Seq) bool {
	if len(seq) >= 1 {
		//XXX fragile
		_, ok1 := seq[0].(*dag.FileScan)
		_, ok2 := seq[0].(*dag.PoolScan)
		return !(ok1 || ok2)
	}
	return true
}

func (a *analyzer) evalPositiveInteger(e ast.Expr) int {
	expr := a.semExpr(e)
	val, err := rungen.EvalAtCompileTime(a.sctx, expr)
	if err != nil {
		a.error(e, err)
		return -1
	}
	if !super.IsInteger(val.Type().ID()) || val.IsNull() {
		a.error(e, fmt.Errorf("expression value must be an integer value: %s", sup.FormatValue(val)))
		return -1
	}
	v := int(val.AsInt())
	if v < 0 {
		a.error(e, errors.New("expression value must be a positive integer"))
	}
	return v
}
