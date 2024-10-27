package zfmt

import (
	"fmt"
	"slices"

	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/zson"
)

func AST(p ast.Seq) string {
	c := &canon{canonZed: canonZed{formatter{tab: 2}}, head: true, first: true}
	if scope, ok := p[0].(*ast.Scope); ok {
		c.scope(scope, false)
	} else {
		c.seq(p)
	}
	c.flush()
	return c.String()
}

type canon struct {
	canonZed
	head  bool
	first bool
}

func (c *canon) open(args ...interface{}) {
	c.formatter.open(args...)
}

func (c *canon) close() {
	c.formatter.close()
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

func (c *canon) defs(defs []ast.Def, separator string) {
	for k, d := range defs {
		if k > 0 {
			c.write(separator)
		}
		c.def(d)
	}
}

func (c *canon) def(d ast.Def) {
	c.write("%s=", d.Name.Name)
	c.expr(d.Expr, "")
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
		c.write("%s(", e.Name)
		if e.Expr != nil {
			c.expr(e.Expr, "")
		}
		c.write(")")
		if e.Where != nil {
			c.write(" where ")
			c.expr(e.Where, "")
		}
	case *ast.Assignment:
		c.assignment(*e)
	case *ast.Primitive:
		c.literal(*e)
	case *ast.ID:
		c.write(e.Name)
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
	case *ast.Grep:
		c.write("grep(")
		c.expr(e.Pattern, "")
		if e.Expr != nil {
			c.write(",")
			c.expr(e.Expr, "")
		}
		c.write(")")
	case *ast.IndexExpr:
		c.expr(e.Expr, "")
		c.write("[")
		c.expr(e.Index, "")
		c.write("]")
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
				c.write(zson.QuotedName(e.Name.Text))
				c.write(":")
				c.expr(e.Value, "")
			case *ast.ID:
				c.write(zson.QuotedName(e.Name))
			case *ast.Spread:
				c.write("...")
				c.expr(e.Expr, "")
			default:
				c.write("zfmt: unknown record elem type: %T", e)
			}
		}
		c.write("}")
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
	case *ast.OverExpr:
		c.open("(")
		c.ret()
		c.write("over ")
		c.exprs(e.Exprs)
		if len(e.Locals) > 0 {
			c.write(" with ")
			c.defs(e.Locals, ", ")
		}
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
		c.write("type %s = ", zson.QuotedName(d.Name.Name))
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
		c.open("fork (")
		for _, p := range p.Paths {
			c.ret()
			c.write("=>")
			c.open()
			c.head = true
			c.seq(p)
			c.close()
		}
		c.close()
		c.ret()
		c.flush()
		c.write(")")
	case *ast.Switch:
		c.next()
		c.write("switch ")
		if p.Expr != nil {
			c.expr(p.Expr, "")
			c.write(" ")
		}
		c.open("(")
		for _, k := range p.Cases {
			c.ret()
			if k.Expr != nil {
				c.write("case ")
				c.expr(k.Expr, "")
			} else {
				c.write("default")
			}
			c.write(" =>")
			c.open()
			c.head = true
			c.seq(k.Path)
			c.close()
		}
		c.close()
		c.ret()
		c.flush()
		c.write(")")
	case *ast.From:
		c.next()
		c.open("from (")
		for _, seq := range p.Trunks {
			c.ret()
			c.source(seq[0].(ast.Source))
			if len(seq) > 1 {
				c.write(" =>")
				c.open()
				c.head = true
				c.seq(seq[1:])
				c.close()
			}
		}
		c.close()
		c.ret()
		c.flush()
		c.write(")")
	case *ast.Pool:
		c.next()
		c.open("")
		c.write("from ")
		c.pool(p)
		c.close()
	case *ast.File:
		c.next()
		c.open("")
		c.file(p)
		c.close()
	case *ast.HTTP:
		c.next()
		c.open("")
		c.http(p)
		c.close()
	case *ast.Summarize:
		c.next()
		c.open("summarize")
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
	case *ast.Cut:
		c.next()
		c.write("cut ")
		c.assignments(p.Args)
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
		if p.NullsFirst {
			c.write(" -nulls first")
		}
		for k, s := range p.Args {
			if k > 0 {
				c.write(",")
			}
			c.space()
			c.expr(s.Expr, "")
			if s.Order != nil {
				c.write(" %s", s.Order.Name)
			}
		}
	case *ast.Load:
		c.next()
		c.write("load %s", zson.QuotedString([]byte(p.Pool.Text)))
		if p.Branch != nil {
			c.write("@%s", p.Branch.Text)
		}
		if p.Author != nil {
			c.write(" author %s", p.Author.Text)
		}
		if p.Message != nil {
			c.write(" message %s", p.Message.Text)
		}
		if p.Meta != nil {
			c.write(" meta %s", p.Meta.Text)
		}
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
			which = "yield "
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
		c.write("top limit=%d flush=%t ", p.Limit, p.Flush)
		c.exprs(p.Args)
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
		c.write("join ")
		if p.RightInput != nil {
			c.open("(")
			c.head = true
			c.seq(p.RightInput)
			c.close()
			c.ret()
			c.flush()
			c.write(") ")
		}
		c.write("on ")
		c.expr(p.LeftKey, "")
		if p.RightKey != nil {
			c.write("=")
			c.expr(p.RightKey, "")
		}
		if p.Args != nil {
			c.write(" ")
			c.assignments(p.Args)
		}
	case *ast.OpAssignment:
		c.next()
		which := "put "
		if isAggAssignments(p.Assignments) {
			which = "summarize "
		}
		c.open(which)
		c.assignments(p.Assignments)
		c.close()
	case *ast.Merge:
		c.next()
		c.write("merge ")
		c.expr(p.Expr, "")
	case *ast.Over:
		c.over(p)
	case *ast.Yield:
		c.next()
		c.write("yield ")
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
	default:
		c.open("unknown proc: %T", p)
		c.close()
	}
}

func (c *canon) over(o *ast.Over) {
	c.next()
	c.write("over ")
	c.exprs(o.Exprs)
	if len(o.Locals) > 0 {
		c.write(" with ")
		c.defs(o.Locals, ", ")
	}
	if o.Body != nil {
		c.write(" => (")
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

func (c *canon) pool(p *ast.Pool) {
	//XXX TBD name, from, to, id etc
	s := pattern(p.Spec.Pool)
	if p.Spec.Commit != nil {
		s += "@" + p.Spec.Commit.Text
	}
	if p.Spec.Meta != nil {
		s += ":" + p.Spec.Meta.Text
	}
	if p.Spec.Tap {
		s += " tap"
	}
	c.write(s)
}

func pattern(p ast.Pattern) string {
	switch p := p.(type) {
	case nil:
		return ""
	case *ast.Glob:
		return p.Pattern
	case *ast.Regexp:
		return "/" + p.Pattern + "/"
	case *ast.Name:
		return zson.QuotedName(p.Text)
	default:
		return fmt.Sprintf("(unknown pattern type %T)", p)
	}
}

func isAggFunc(e ast.Expr) *ast.Summarize {
	call, ok := e.(*ast.Call)
	if !ok {
		return nil
	}
	if _, err := agg.NewPattern(call.Name.Name, true); err != nil {
		return nil
	}
	return &ast.Summarize{
		Kind: "Summarize",
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
	case *ast.Grep, *ast.Regexp, *ast.Glob:
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

func (c *canon) http(p *ast.HTTP) {
	//XXX TBD other stuff
	c.write("get %s", pattern(p.URL))
	if p.Format != nil {
		c.write(" format %s", zson.QuotedName(p.Format.Text))
	}
	if p.Method != nil {
		c.write(" method %s", zson.QuotedName(p.Method.Text))
	}
	if p.Headers != nil {
		c.write(" headers ")
		c.expr(p.Headers, "")
	}
	if p.Body != nil {
		c.write(" body %s", zson.QuotedName(p.Body.Text))
	}
}

func (c *canon) file(p *ast.File) {
	//XXX TBD other stuff
	c.write("file %s", pattern(p.Path))
	if p.Format != nil {
		c.write(" format %s", zson.QuotedName(p.Format.Text))
	}
}

func (c *canon) source(src ast.Source) {
	switch src := src.(type) {
	case *ast.Pool:
		c.write("pool ")
		c.pool(src)
	case *ast.HTTP:
		c.http(src)
	case *ast.File:
		c.file(src)
	default:
		c.write("unknown source type: %T", src)
	}
}
