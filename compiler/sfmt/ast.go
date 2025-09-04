package sfmt

import (
	"fmt"
	"slices"
	"strings"

	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/sup"
)

func AST(p ast.Seq) string {
	if len(p) == 0 {
		return ""
	}
	c := &canon{canonZed: canonZed{formatter{tab: 2}}, head: true, first: true}
	if scope, ok := p[0].(*ast.Scope); ok {
		c.scope(scope, false)
	} else {
		c.seq(p)
	}
	c.flush()
	return c.String()
}

func ASTExpr(e ast.Expr) string {
	d := &canon{
		canonZed: canonZed{formatter: formatter{tab: 2}},
		head:     true,
		first:    true,
	}
	d.expr(e, "")
	d.flush()
	return d.String()
}

type canon struct {
	canonZed
	head  bool
	first bool
}

func (c *canon) assignments(assignments []ast.Assignment) {
	for k, a := range assignments {
		if k > 0 {
			c.write(",")
		}
		c.assignment(a)
	}
}

func (c *canon) assignment(a ast.Assignment) {
	if a.LHS != nil {
		c.expr(a.LHS, "")
		c.write(":=")
	}
	c.expr(a.RHS, "")
}

func (c *canon) exprs(exprs []ast.Expr) {
	for k, e := range exprs {
		if k > 0 {
			c.write(", ")
		}
		c.expr(e, "")
	}
}

func (c *canon) expr(e ast.Expr, parent string) {
	switch e := e.(type) {
	case nil:
		c.write("null")
	case *ast.Agg:
		var distinct string
		if e.Distinct {
			distinct = "distinct "
		}
		c.write("%s(%s", e.Name, distinct)
		if e.Expr != nil {
			c.expr(e.Expr, "")
		}
		c.write(")")
		if e.Where != nil {
			c.write(" where ")
			c.expr(e.Where, "")
		}
	case *ast.SQLAsExpr:
		c.expr(e.Expr, "")
		if e.Label != nil {
			c.write(" as %s", e.Label.Name)
		}
	case *ast.Assignment:
		c.assignment(*e)
	case *ast.Primitive:
		c.literal(*e)
	case *ast.ID:
		c.write(e.Name)
	case *ast.DoubleQuote:
		c.write("%q", e.Text)
	case *ast.UnaryExpr:
		c.write(e.Op)
		c.expr(e.Operand, "not")
	case *ast.BinaryExpr:
		c.binary(e, parent)
	case *ast.Conditional:
		c.write("(")
		c.expr(e.Cond, "")
		c.write(") ? ")
		c.expr(e.Then, "")
		c.write(" : ")
		c.expr(e.Else, "")
	case *ast.Call:
		c.write("%s(", e.Name.Name)
		c.exprs(e.Args)
		c.write(")")
	case *ast.CallExtract:
		c.write("EXTRACT(")
		c.expr(e.Part, "")
		c.write(" FROM ")
		c.expr(e.Expr, "")
		c.write(")")
	case *ast.CaseExpr:
		c.write("case ")
		c.expr(e.Expr, "")
		for _, when := range e.Whens {
			c.write(" when ")
			c.expr(when.Cond, "")
			c.write(" then ")
			c.expr(when.Then, "")
		}
		if e.Else != nil {
			c.write(" else ")
			c.expr(e.Else, "")
		}
		c.write(" end")
	case *ast.Cast:
		c.expr(e.Type, "")
		c.write("(")
		c.expr(e.Expr, "")
		c.write(")")
	case *ast.TypeValue:
		c.write("<")
		c.typ(e.Value)
		c.write(">")
	case *ast.Regexp:
		c.write("/%s/", e.Pattern)
	case *ast.Glob:
		c.write(e.Pattern)
	case *ast.IndexExpr:
		c.expr(e.Expr, "")
		c.write("[")
		c.expr(e.Index, "")
		c.write("]")
	case *ast.IsNullExpr:
		c.expr(e.Expr, "")
		c.write("IS ")
		if e.Not {
			c.write("NOT ")
		}
		c.write("NULL")
	case *ast.SliceExpr:
		c.expr(e.Expr, "")
		c.write("[")
		if e.From != nil {
			c.expr(e.From, "")
		}
		c.write(":")
		if e.To != nil {
			c.expr(e.To, "")
		}
		c.write("]")
	case *ast.Term:
		c.write(e.Text)
	case *ast.RecordExpr:
		c.write("{")
		for k, elem := range e.Elems {
			if k != 0 {
				c.write(",")
			}
			switch e := elem.(type) {
			case *ast.FieldExpr:
				c.write(sup.QuotedName(e.Name.Text))
				c.write(":")
				c.expr(e.Value, "")
			case *ast.ID:
				c.write(sup.QuotedName(e.Name))
			case *ast.Spread:
				c.write("...")
				c.expr(e.Expr, "")
			default:
				c.write("sfmt: unknown record elem type: %T", e)
			}
		}
		c.write("}")
	case *ast.SQLCast:
		c.write("CAST(")
		c.expr(e.Expr, "")
		c.write(" AS ")
		c.typ(e.Type)
		c.write(")")
	case *ast.SQLSubstring:
		c.write("SUBSTRING(")
		c.expr(e.Expr, "")
		if e.From != nil {
			c.write(" FROM ")
			c.expr(e.From, "")
		}
		if e.For != nil {
			c.write(" FOR ")
			c.expr(e.For, "")
		}
		c.write(")")
	case *ast.ArrayExpr:
		c.write("[")
		c.vectorElems(e.Elems)
		c.write("]")
	case *ast.SetExpr:
		c.write("|[")
		c.vectorElems(e.Elems)
		c.write("]|")
	case *ast.MapExpr:
		c.write("|{")
		for k, e := range e.Entries {
			if k != 0 {
				c.write(",")
			}
			c.expr(e.Key, "")
			c.write(":")
			c.expr(e.Value, "")
		}
		c.write("}|")
	case *ast.Subquery:
		c.open("(")
		c.head = true
		c.seq(e.Body)
		c.close()
		c.ret()
		c.flush()
		c.write(")")
	case *ast.Exists:
		c.open("exists(")
		c.head = true
		c.seq(e.Body)
		c.close()
		c.ret()
		c.flush()
		c.write(")")
	case *ast.FString:
		c.write(`f"`)
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *ast.FStringExpr:
				c.write("{")
				c.expr(elem.Expr, "")
				c.write("}")
			case *ast.FStringText:
				c.write(elem.Text)
			default:
				c.write("(unknown f-string element %T)", elem)
			}
		}
		c.write(`"`)
	case *ast.Between:
		c.write("(")
		c.expr(e.Expr, "")
		if e.Not {
			c.write(" not")
		}
		c.write(" between ")
		c.expr(e.Lower, "")
		c.write(" and ")
		c.expr(e.Upper, "")
		c.write(")")
	case *ast.SQLTimeValue:
		c.write("%s %s", strings.ToUpper(e.Type), sup.QuotedString(e.Value.Text))
	case *ast.TupleExpr:
		c.write("(")
		c.exprs(e.Elems)
		c.write(")")
	default:
		c.write("(unknown expr %T)", e)
	}
}

func (c *canon) vectorElems(elems []ast.VectorElem) {
	for k, elem := range elems {
		if k > 0 {
			c.write(",")
		}
		switch elem := elem.(type) {
		case *ast.Spread:
			c.write("...")
			c.expr(elem.Expr, "")
		case *ast.VectorValue:
			c.expr(elem.Expr, "")
		}
	}
}

func (c *canon) binary(e *ast.BinaryExpr, parent string) {
	switch e.Op {
	case ".":
		if !isThis(e.LHS) {
			c.expr(e.LHS, "")
			c.write(".")
		}
		c.expr(e.RHS, "")
	case "and", "or", "in":
		parens := needsparens(parent, e.Op)
		c.maybewrite("(", parens)
		c.expr(e.LHS, e.Op)
		c.write(" %s ", e.Op)
		c.expr(e.RHS, e.Op)
		c.maybewrite(")", parens)
	default:
		parens := needsparens(parent, e.Op)
		c.maybewrite("(", parens)
		// do need parens calc
		c.expr(e.LHS, e.Op)
		c.write("%s", e.Op)
		c.expr(e.RHS, e.Op)
		c.maybewrite(")", parens)
	}
}

func needsparens(parent, op string) bool {
	return precedence(parent)-precedence(op) < 0
}

func precedence(op string) int {
	switch op {
	case "not":
		return 1
	case "^":
		return 2
	case "*", "/", "%":
		return 3
	case "+", "-":
		return 4
	case "<", "<=", ">", ">=", "==", "!=", "in":
		return 5
	case "and":
		return 6
	case "or":
		return 7
	default:
		return 100
	}
}

func isThis(e ast.Expr) bool {
	if id, ok := e.(*ast.ID); ok {
		return id.Name == "this"
	}
	return false
}

func (c *canon) maybewrite(s string, do bool) {
	if do {
		c.write(s)
	}
}

func (c *canon) next() {
	if c.first {
		c.first = false
	} else {
		c.write("\n")
	}
	c.needRet = false
	c.writeTab()
	if c.head {
		c.head = false
	} else {
		c.write("| ")
	}
}

func (c *canon) decl(d ast.Decl) {
	switch d := d.(type) {
	case *ast.ConstDecl:
		c.write("const %s = ", d.Name.Name)
		c.expr(d.Expr, "")
	case *ast.FuncDecl:
		c.write("func %s(", d.Name.Name)
		for i := range d.Params {
			if i != 0 {
				c.write(", ")
			}
			c.write(d.Params[i].Name)
		}
		c.open("): (")
		c.ret()
		c.expr(d.Expr, d.Name.Name)
		c.close()
		c.ret()
		c.flush()
		c.write(")")
	case *ast.OpDecl:
		c.write("op %s(", d.Name.Name)
		for k, p := range d.Params {
			if k > 0 {
				c.write(", ")
			}
			c.write(p.Name)
		}
		c.open("): (")
		c.ret()
		c.flush()
		c.head = true
		c.seq(d.Body)
		c.close()
		c.ret()
		c.flush()
		c.write(")")
		c.head, c.first = true, true
	case *ast.TypeDecl:
		c.write("type %s = ", sup.QuotedName(d.Name.Name))
		c.typ(d.Type)
	default:
		c.open("unknown decl: %T", d)
		c.close()
	}

}

func (c *canon) seq(seq ast.Seq) {
	for _, p := range seq {
		c.op(p)
	}
}

func (c *canon) op(p ast.Op) {
	switch p := p.(type) {
	case *ast.Scope:
		c.scope(p, true)
	case *ast.Parallel:
		c.next()
		c.open("fork")
		for _, p := range p.Paths {
			c.ret()
			c.write("(")
			c.open()
			c.head = true
			c.seq(p)
			c.close()
			c.ret()
			c.write(")")
		}
		c.close()
		c.flush()
	case *ast.Switch:
		c.next()
		c.write("switch")
		if p.Expr != nil {
			c.write(" ")
			c.expr(p.Expr, "")
		}
		for _, k := range p.Cases {
			c.ret()
			if k.Expr != nil {
				c.write("case ")
				c.expr(k.Expr, "")
			} else {
				c.write("default")
			}
			c.write(" (")
			c.open()
			c.head = true
			c.seq(k.Path)
			c.close()
			c.ret()
			c.write(")")
		}
		c.close()
		c.flush()
	case *ast.From:
		c.next()
		c.write("from ")
		c.fromElems(p.Elems)
	case *ast.Aggregate:
		c.next()
		c.open("aggregate")
		c.ret()
		c.open()
		c.assignments(p.Aggs)
		if len(p.Keys) != 0 {
			c.write(" by ")
			c.assignments(p.Keys)
		}
		if p.Limit != 0 {
			c.write(" -with limit %d", p.Limit)
		}
		c.close()
		c.close()
	case *ast.CallOp:
		c.next()
		c.write("call %s ", sup.QuotedName(p.Name.Name))
		c.exprs(p.Args)
	case *ast.Cut:
		c.next()
		c.write("cut ")
		c.assignments(p.Args)
	case *ast.Distinct:
		c.next()
		c.write("distinct ")
		c.expr(p.Expr, "")
	case *ast.Drop:
		c.next()
		c.write("drop ")
		c.exprs(p.Args)
	case *ast.Sort:
		c.next()
		c.write("sort")
		if p.Reverse {
			c.write(" -r")
		}
		c.sortExprs(p.Exprs)
	case *ast.Load:
		c.next()
		c.write("load %s", sup.QuotedString(p.Pool.Text))
		c.opArgs(p.Args)
	case *ast.Head:
		c.next()
		c.open("head")
		if p.Count != nil {
			c.write(" ")
			c.expr(p.Count, "")
		}
		c.close()
	case *ast.Tail:
		c.next()
		c.open("tail")
		if p.Count != nil {
			c.write(" ")
			c.expr(p.Count, "")
		}
		c.close()
	case *ast.Uniq:
		c.next()
		c.write("uniq")
		if p.Cflag {
			c.write(" -c")
		}
	case *ast.Pass:
		c.next()
		c.write("pass")
	case *ast.OpExpr:
		if agg := isAggFunc(p.Expr); agg != nil {
			c.op(agg)
			return
		}
		c.next()
		var which string
		e := p.Expr
		if IsSearch(e) {
			which = "search "
		} else if IsBool(e) {
			which = "where "
		} else if _, ok := e.(*ast.Call); !ok {
			which = "values "
		}
		// Since we can't determine whether the expression is a func call or
		// an op call until the semantic pass, leave this ambiguous.
		// XXX (nibs) - I don't think we should be doing this kind introspection
		// here. This is why we have the semantic pass and canonical zed here
		// should reflect the ambiguous nature of the expression.
		if which != "" {
			c.open(which)
			defer c.close()
		}
		c.expr(e, "")
	case *ast.Search:
		c.next()
		c.open("search ")
		c.expr(p.Expr, "")
		c.close()
	case *ast.Where:
		c.next()
		c.open("where ")
		c.expr(p.Expr, "")
		c.close()
	case *ast.Top:
		c.next()
		c.write("top")
		if p.Reverse {
			c.write(" -r")
		}
		if p.Limit != nil {
			c.write(" ")
			c.expr(p.Limit, "")
		}
		c.sortExprs(p.Exprs)
	case *ast.Put:
		c.next()
		c.write("put ")
		c.assignments(p.Args)
	case *ast.Rename:
		c.next()
		c.write("rename ")
		c.assignments(p.Args)
	case *ast.Fuse:
		c.next()
		c.write("fuse")
	case *ast.Join:
		c.next()
		if p.Style != "" {
			c.write("%s ", p.Style)
		}
		c.write("join")
		if p.RightInput != nil {
			c.open(" (")
			c.head = true
			c.seq(p.RightInput)
			c.close()
			c.ret()
			c.flush()
			c.write(")")
		}
		if p.Alias != nil {
			c.write(" as {%s,%s}", p.Alias.Left.Name, p.Alias.Right.Name)
		}
		c.joinCond(p.Cond)
	case *ast.OpAssignment:
		c.next()
		which := "put "
		if isAggAssignments(p.Assignments) {
			which = "aggregate "
		}
		c.open(which)
		c.assignments(p.Assignments)
		c.close()
	case *ast.Merge:
		c.next()
		c.write("merge")
		c.sortExprs(p.Exprs)
	case *ast.Unnest:
		c.unnest(p)
	case *ast.Values:
		c.next()
		c.write("values ")
		c.exprs(p.Exprs)
	case *ast.SQLValues:
		c.next()
		c.write("values ")
		c.exprs(p.Exprs)
	case *ast.Output:
		c.next()
		c.write("output %s", p.Name.Name)
	case *ast.Debug:
		c.next()
		c.write("debug")
		if p.Expr != nil {
			c.write(" ")
			c.expr(p.Expr, "")
		}
	case *ast.SQLLimitOffset:
		c.op(p.Op)
		if p.Limit != nil {
			c.ret()
			c.write("limit ")
			c.expr(p.Limit, "")
		}
		if p.Offset != nil {
			c.ret()
			c.write("offset ")
			c.expr(p.Offset, "")
		}
	case *ast.SQLOrderBy:
		c.op(p.Op)
		c.ret()
		c.write("order by")
		c.sortExprs(p.Exprs)
	case *ast.SQLPipe:
		c.next()
		c.open("(")
		c.head = true
		c.seq(p.Ops)
		c.close()
		c.ret()
		c.flush()
		c.write(")")
	case *ast.SQLSelect:
		c.next()
		c.write("select ")
		if p.Distinct {
			c.write("distinct ")
		}
		if p.Value {
			c.write("value ")
		}
		for i, a := range p.Selection.Args {
			if i > 0 {
				c.write(", ")
			}
			c.expr(&a, "")
		}
		if p.From != nil {
			c.head = true
			c.op(p.From)
		}
		if p.Where != nil {
			c.ret()
			c.write("where ")
			c.expr(p.Where, "")
		}
		if len(p.GroupBy) > 0 {
			c.ret()
			c.write("group by ")
			c.exprs(p.GroupBy)
		}
		if p.Having != nil {
			c.ret()
			c.write("having ")
			c.expr(p.Having, "")
		}
	case *ast.SQLUnion:
		c.op(p.Left)
		c.ret()
		c.write("union")
		if p.Distinct {
			c.write(" distinct")
		} else {
			c.write(" all")
		}
		c.head = true
		c.op(p.Right)
	case *ast.SQLWith:
		c.next()
		c.write("with ")
		if p.Recursive {
			c.write("recursive ")
		}
		for i, cte := range p.CTEs {
			if i > 0 {
				c.write(", ")
			}
			c.write("%s as ", cte.Name.Name)
			if cte.Materialized {
				c.write("materialized ")
			}
			c.first, c.head = true, true
			c.op(cte.Body)
		}
		c.head = true
		c.op(p.Body)
	default:
		c.open("unknown operator: %T", p)
		c.close()
	}
}

func (c *canon) fromElems(elems []*ast.FromElem) {
	c.fromElem(elems[0])
	for _, elem := range elems[1:] {
		c.write(", ")
		c.fromElem(elem)
	}
}

func (c *canon) fromElem(elem *ast.FromElem) {
	c.fromEntity(elem.Entity)
	c.opArgs(elem.Args)
	if elem.Alias != nil {
		c.tableAlias(elem.Alias)
	}
}

func (c *canon) tableAlias(alias *ast.TableAlias) {
	c.write(" as %s", sup.QuotedName(alias.Name))
	if len(alias.Columns) != 0 {
		c.write(" (")
		var comma string
		for _, col := range alias.Columns {
			c.write("%s%s", comma, sup.QuotedName(col.Name))
			comma = ", "
		}
		c.write(")")
	}
}

func (c *canon) fromEntity(e ast.FromEntity) {
	switch e := e.(type) {
	case *ast.ExprEntity:
		c.write("eval(")
		c.expr(e.Expr, "")
		c.write(")")
	case *ast.Glob, *ast.Regexp:
		c.pattern(e)
	case *ast.Text:
		c.write(sup.QuotedName(e.Text))
	case *ast.SQLCrossJoin:
		c.fromElem(e.Left)
		c.ret()
		c.write("cross join ")
		c.fromElem(e.Right)
	case *ast.SQLJoin:
		c.fromElem(e.Left)
		c.ret()
		if e.Style != "" {
			c.write(e.Style + " ")
		}
		c.write("join ")
		c.fromElem(e.Right)
		c.joinCond(e.Cond)
	default:
		panic(fmt.Sprintf("unknown from expression: %T", e))
	}
}

func (c *canon) joinCond(e ast.JoinCond) {
	switch e := e.(type) {
	case *ast.JoinOnCond:
		c.write(" on ")
		c.expr(e.Expr, "")
	case *ast.JoinUsingCond:
		c.write(" using (")
		c.exprs(e.Fields)
		c.write(")")
	default:
		panic(e)
	}
}

func (c *canon) unnest(o *ast.Unnest) {
	c.next()
	c.write("unnest ")
	c.expr(o.Expr, "")
	if o.Body != nil {
		c.write(" into (")
		c.open()
		c.head = true
		c.seq(o.Body)
		c.close()
		c.ret()
		c.flush()
		c.write(")")
	}
}

func (c *canon) scope(s *ast.Scope, parens bool) {
	if parens {
		c.open("(")
		c.ret()
	}
	for _, d := range s.Decls {
		c.decl(d)
		c.ret()
	}
	//XXX functions?
	c.flush()
	c.seq(s.Body)
	if parens {
		c.close()
		c.ret()
		c.flush()
		c.write(")")
	}
}

func (c *canon) sortExprs(sortExprs []ast.SortExpr) {
	for i, s := range sortExprs {
		if i > 0 {
			c.write(",")
		}
		c.space()
		c.expr(s.Expr, "")
		if s.Order != nil {
			c.write(" %s", s.Order.Name)
		}
		if s.Nulls != nil {
			c.write(" nulls %s", s.Nulls.Name)
		}
	}
}

func (c *canon) opArgs(args []ast.OpArg) {
	if len(args) == 0 {
		return
	}
	c.write(" (")
	for _, arg := range args {
		switch arg := arg.(type) {
		case *ast.ArgText:
			c.write(" %s %s", arg.Key, sup.QuotedName(arg.Value.Text))
		case *ast.ArgExpr:
			c.write(" %s ", arg.Key)
			c.expr(arg.Value, "")
		default:
			panic("fromArgs")
		}
	}
	c.write(" )")
}

func (c *canon) pattern(p ast.FromEntity) {
	switch p := p.(type) {
	case *ast.Glob:
		c.write(p.Pattern)
	case *ast.Regexp:
		c.write("/" + p.Pattern + "/")
	default:
		panic(fmt.Sprintf("(unknown pattern type %T)", p))
	}
}

func isAggFunc(e ast.Expr) *ast.Aggregate {
	call, ok := e.(*ast.Call)
	if !ok {
		return nil
	}
	if _, err := agg.NewPattern(call.Name.Name, false, true); err != nil {
		return nil
	}
	return &ast.Aggregate{
		Kind: "aggregate",
		Aggs: []ast.Assignment{{
			Kind: "Assignment",
			RHS:  call,
		}},
	}
}

func IsBool(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Primitive:
		return e.Type == "bool"
	case *ast.UnaryExpr:
		return IsBool(e.Operand)
	case *ast.BinaryExpr:
		switch e.Op {
		case "and", "or", "in", "==", "!=", "<", "<=", ">", ">=":
			return true
		default:
			return false
		}
	case *ast.Conditional:
		return IsBool(e.Then) && IsBool(e.Else)
	case *ast.Call:
		return function.HasBoolResult(e.Name.Name)
	case *ast.Cast:
		if typval, ok := e.Type.(*ast.TypeValue); ok {
			if typ, ok := typval.Value.(*ast.TypePrimitive); ok {
				return typ.Name == "bool"
			}
		}
		return false
	case *ast.Regexp, *ast.Glob:
		return true
	default:
		return false
	}
}

func isAggAssignments(assigns []ast.Assignment) bool {
	return !slices.ContainsFunc(assigns, func(a ast.Assignment) bool {
		return isAggFunc(a.RHS) == nil
	})
}

func IsSearch(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Regexp, *ast.Glob, *ast.Term:
		return true
	case *ast.BinaryExpr:
		switch e.Op {
		case "and", "or":
			return IsSearch(e.LHS) || IsSearch(e.RHS)
		default:
			return false
		}
	case *ast.UnaryExpr:
		return IsSearch(e.Operand)
	default:
		return false
	}
}
