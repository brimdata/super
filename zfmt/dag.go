package zfmt

import (
	"fmt"

	"github.com/brimdata/zed/compiler/ast/dag"
	astzed "github.com/brimdata/zed/compiler/ast/zed"
	"github.com/brimdata/zed/compiler/kernel"
	"github.com/brimdata/zed/order"
)

func DAG(op dag.Op) string {
	d := &canonDAG{
		canonZed: canonZed{formatter: formatter{tab: 2}},
		head:     true,
		first:    true,
	}
	d.op(op)
	d.flush()
	return d.String()
}

type canonDAG struct {
	canonZed
	head  bool
	first bool
}

func (c *canonDAG) open(args ...interface{}) {
	c.formatter.open(args...)
}

func (c *canonDAG) close() {
	c.formatter.close()
}

func (c *canonDAG) assignments(assignments []dag.Assignment) {
	for k, a := range assignments {
		if k > 0 {
			c.write(",")
		}
		if a.LHS != nil {
			c.expr(a.LHS, false)
			c.write(":=")
		}
		c.expr(a.RHS, false)
	}
}

func (c *canonDAG) indexPredicates(predicates []dag.IndexPredicate) {
	for k, p := range predicates {
		if k > 0 {
			c.write(", ")
		}
		c.expr(p.Key, false)
		c.write("==")
		c.expr(p.Value, false)
	}
}

func (c *canonDAG) exprs(exprs []dag.Expr) {
	for k, e := range exprs {
		if k > 0 {
			c.write(", ")
		}
		c.expr(e, false)
	}
}

func (c *canonDAG) expr(e dag.Expr, paren bool) {
	switch e := e.(type) {
	case nil:
		c.write("null")
	case *dag.Agg:
		c.write("%s(", e.Name)
		if e.Expr != nil {
			c.expr(e.Expr, false)
		}
		c.write(")")
		if e.Where != nil {
			c.write(" where ")
			c.expr(e.Where, false)
		}
	case *astzed.Primitive:
		c.literal(*e)
	case *dag.UnaryExpr:
		c.space()
		c.write(e.Op)
		c.expr(e.Operand, true)
	case *dag.SelectExpr:
		c.write("TBD:select")
	case *dag.BinaryExpr:
		c.binary(e)
	case *dag.Conditional:
		c.write("(")
		c.expr(e.Cond, true)
		c.write(") ? ")
		c.expr(e.Then, false)
		c.write(" : ")
		c.expr(e.Else, false)
	case *dag.Call:
		c.write("%s(", e.Name)
		c.exprs(e.Args)
		c.write(")")
	case *dag.Cast:
		c.expr(e.Expr, false)
		c.open(":%s", e.Type)
	case *dag.Search:
		c.write("match(")
		c.literal(e.Value)
		c.write(")")
	case *dag.Path:
		c.fieldpath(e.Name)
	case *dag.Ref:
		c.write("%s", e.Name)
	case *astzed.TypeValue:
		c.write("type(")
		c.typ(e.Value)
		c.write(")")
	default:
		c.open("(unknown expr %T)", e)
		c.close()
		c.ret()
	}
}

func (c *canonDAG) binary(e *dag.BinaryExpr) {
	switch e.Op {
	case ".":
		if !isDAGRoot(e.LHS) {
			c.expr(e.LHS, false)
			c.write(".")
		}
		c.expr(e.RHS, false)
	case "[":
		if isDAGRoot(e.LHS) {
			c.write(".")
		} else {
			c.expr(e.LHS, false)
		}
		c.write("[")
		c.expr(e.RHS, false)
		c.write("]")
	case "in", "and":
		c.expr(e.LHS, false)
		c.write(" %s ", e.Op)
		c.expr(e.RHS, false)
	case "or":
		c.expr(e.LHS, true)
		c.write(" %s ", e.Op)
		c.expr(e.RHS, true)
	default:
		// do need parens calc
		c.expr(e.LHS, true)
		c.write("%s", e.Op)
		c.expr(e.RHS, true)
	}
}

func isDAGRoot(e dag.Expr) bool {
	if f, ok := e.(*dag.Path); ok {
		if f.Name != nil && len(f.Name) == 0 {
			return true
		}
	}
	return false
}

func (c *canonDAG) next() {
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

func (c *canonDAG) op(p dag.Op) {
	switch p := p.(type) {
	case *dag.Sequential:
		if p == nil {
			return
		}
		for _, p := range p.Ops {
			c.op(p)
		}
	case *dag.Parallel:
		c.next()
		c.open("split (")
		for _, p := range p.Ops {
			c.ret()
			c.write("=>")
			c.open()
			c.head = true
			c.op(p)
			c.close()
		}
		c.close()
		c.ret()
		c.flush()
		c.write(")")
	case *dag.Switch:
		c.next()
		c.open("switch ")
		if p.Expr != nil {
			c.expr(p.Expr, false)
			c.write(" ")
		}
		c.open("(")
		for _, k := range p.Cases {
			c.ret()
			if k.Expr != nil {
				c.expr(k.Expr, false)
			} else {
				c.write("default")
			}
			c.write(" =>")
			c.open()
			c.head = true
			c.op(k.Op)
			c.close()
		}
		c.close()
		c.ret()
		c.flush()
		c.write(")")
	case *dag.Merge:
		c.next()
		c.write("merge ")
		c.fieldpath(p.Key)
		c.write(":" + p.Order.String())
	case *dag.Const:
		c.write("const %s=", p.Name)
		c.expr(p.Expr, false)
		c.ret()
		c.flush()
	case *dag.TypeProc:
		c.write("type %s=", p.Name)
		c.typ(p.Type)
		c.ret()
		c.flush()
	case *dag.Summarize:
		c.next()
		c.open("summarize")
		if p.Duration != nil {
			c.write(" every ")
			c.literal(*p.Duration)
		}
		if p.PartialsIn {
			c.write(" partials-in")
		}
		if p.PartialsOut {
			c.write(" partials-out")
		}
		if p.InputSortDir != 0 {
			c.write(" sort-dir %d", p.InputSortDir)
		}
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
	case *dag.Cut:
		c.next()
		c.write("cut ")
		c.assignments(p.Args)
	case *dag.Pick:
		c.next()
		c.open("pick ")
		c.assignments(p.Args)
	case *dag.Drop:
		c.next()
		c.write("drop ")
		c.exprs(p.Args)
	case *dag.Sort:
		c.next()
		c.write("sort")
		if p.Order == order.Desc {
			c.write(" -r")
		}
		if p.NullsFirst {
			c.write(" -nulls first")
		}
		if len(p.Args) > 0 {
			c.space()
			c.exprs(p.Args)
		}
	case *dag.Head:
		c.next()
		c.write("head %d", p.Count)
	case *dag.Tail:
		c.next()
		c.write("tail %d", p.Count)
	case *dag.Uniq:
		c.next()
		c.write("uniq")
		if p.Cflag {
			c.write(" -c")
		}
	case *dag.Pass:
		c.next()
		c.write("pass")
	case *dag.Filter:
		c.next()
		c.open("filter ")
		if isDAGTrue(p.Expr) {
			c.write("*")
		} else {
			c.expr(p.Expr, false)
		}
		c.close()
	case *dag.Top:
		c.next()
		c.write("top limit=%d flush=%t ", p.Limit, p.Flush)
		c.exprs(p.Args)
	case *dag.Put:
		c.next()
		c.write("put ")
		c.assignments(p.Args)
	case *dag.Rename:
		c.next()
		c.write("rename ")
		c.assignments(p.Args)
	case *dag.Fuse:
		c.next()
		c.write("fuse")
	case *dag.Join:
		c.next()
		c.open("join on ")
		c.expr(p.LeftKey, false)
		c.write("=")
		c.expr(p.RightKey, false)
		if len(p.Args) != 0 {
			c.write(" ")
			c.assignments(p.Args)
		}
		c.close()
	case *dag.From:
		// XXX cleanup for single trunk
		c.next()
		c.open("from (")
		for _, trunk := range p.Trunks {
			c.ret()
			if trunk.Pushdown.Scan != nil || trunk.Pushdown.Index != nil {
				c.open("(pushdown")
				if trunk.Pushdown.Scan != nil {
					c.head = true
					c.ret()
					c.open("(scan")
					c.op(trunk.Pushdown.Scan)
					c.write(")")
					c.close()
				}
				if trunk.Pushdown.Index != nil {
					c.head = true
					c.ret()
					c.open("(index ")
					c.indexPredicates(trunk.Pushdown.Index)
					c.write(")")
					c.close()
				}
				c.write(")")
				c.close()
				c.ret()
			}
			c.write("%s", source(trunk.Source))
			if trunk.Seq != nil && len(trunk.Seq.Ops) != 0 {
				c.open()
				c.head = true
				c.write(" =>")
				c.op(trunk.Seq)
				c.close()
			}
			c.write(";")
		}
		c.ret()
		c.close()
		c.write(")")
	default:
		c.open("unknown proc: %T", p)
		c.close()
	}
}

func source(src dag.Source) string {
	switch p := src.(type) {
	case *dag.File:
		s := fmt.Sprintf("file %s", p.Path)
		if p.Format != "" {
			s += fmt.Sprintf(" format %s", p.Format)
		}
		if !p.Layout.IsNil() {
			s += fmt.Sprintf(" order %s", p.Layout)
		}
		return s
	case *dag.HTTP:
		return fmt.Sprintf("get %s", p.URL)
	case *dag.Pool:
		return fmt.Sprintf("%s", p.ID)
	case *dag.PoolMeta:
		return fmt.Sprintf("%s:%s", p.ID, p.Meta)
	case *dag.CommitMeta:
		return fmt.Sprintf("%s@%s:%s", p.Pool, p.Commit, p.Meta)
	case *dag.LakeMeta:
		return fmt.Sprintf(":%s", p.Meta)
		//XXX from, to, order
	case *kernel.Reader:
		return "(internal reader)"
	default:
		return fmt.Sprintf("unknown source %T", p)
	}
}

func isDAGTrue(e dag.Expr) bool {
	if p, ok := e.(*astzed.Primitive); ok {
		return p.Type == "bool" && p.Text == "true"
	}
	return false
}
