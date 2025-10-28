package semantic

import (
	"errors"
	"slices"
	"strings"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/semantic/sem"
)

type dagen struct {
	reporter
	outputs map[*dag.OutputOp]*sem.DebugOp
	funcs   map[string]*dag.FuncDef
}

func newDagen(r reporter) *dagen {
	return &dagen{
		reporter: r,
		outputs:  make(map[*dag.OutputOp]*sem.DebugOp),
		funcs:    make(map[string]*dag.FuncDef),
	}
}

func (d *dagen) assemble(seq sem.Seq, funcs map[string]*funcDef) *dag.Main {
	dagSeq := d.seq(seq)
	dagSeq = d.checkOutputs(true, dagSeq)
	dagFuncs := make([]*dag.FuncDef, 0, len(d.funcs))
	for _, f := range funcs {
		dagFuncs = append(dagFuncs, d.fn(f))
	}
	// Sort function entries so they are consistently ordered by integer tag strings.
	slices.SortFunc(dagFuncs, func(a, b *dag.FuncDef) int {
		return strings.Compare(a.Tag, b.Tag)
	})
	return &dag.Main{Funcs: dagFuncs, Body: dagSeq}
}

func (d *dagen) assembleExpr(e sem.Expr, funcs map[string]*funcDef) *dag.MainExpr {
	dagExpr := d.expr(e)
	dagFuncs := make([]*dag.FuncDef, 0, len(d.funcs))
	for _, f := range funcs {
		dagFuncs = append(dagFuncs, d.fn(f))
	}
	return &dag.MainExpr{Funcs: dagFuncs, Expr: dagExpr}
}

func (d *dagen) seq(seq sem.Seq) dag.Seq {
	var out dag.Seq
	for k, op := range seq {
		// XXX This way of handilng DebugOp doesn't seem right.  It might be cleaner
		// to have a runtime output entity that concurrently pulls from all
		// the debug channels (guaranteeing no deadlock) and pulls until
		// all become blocked and main DAG is done.  If you know the main DAG
		// is done and the DebugOps are all blocked, then you know there will be
		// no more debug values coming.
		if debugOp, ok := op.(*sem.DebugOp); ok {
			return d.debugOp(debugOp, seq[k+1:], out)
		}
		out = append(out, d.op(op))
	}
	return out
}

func (d *dagen) op(op sem.Op) dag.Op {
	switch op := op.(type) {
	//
	/// Scanners in alphabetical order
	//
	case *sem.CommitMetaScan:
		return &dag.CommitMetaScan{
			Kind:   "CommitMetaScan",
			Pool:   op.Pool,
			Commit: op.Commit,
			Meta:   op.Meta,
			Tap:    op.Tap,
		}
	case *sem.DBMetaScan:
		return &dag.DBMetaScan{
			Kind: "DBMetaScan",
			Meta: op.Meta,
		}
	case *sem.DefaultScan:
		return &dag.DefaultScan{
			Kind: "DefaultScan",
		}
	case *sem.DeleteScan:
		return &dag.DeleteScan{
			Kind:   "DeleteScan",
			ID:     op.ID,
			Commit: op.Commit,
		}
	case *sem.FileScan:
		return &dag.FileScan{
			Kind:   "FileScan",
			Paths:  op.Paths,
			Format: op.Format,
		}
	case *sem.HTTPScan:
		return &dag.HTTPScan{
			Kind:    "HTTPScan",
			URL:     op.URL,
			Format:  op.Format,
			Method:  op.Method,
			Headers: op.Headers,
			Body:    op.Body,
		}
	case *sem.NullScan:
		return &dag.NullScan{
			Kind: "NullScan",
		}
	case *sem.PoolMetaScan:
		return &dag.PoolMetaScan{
			Kind: "PoolMetaScan",
			ID:   op.ID,
			Meta: op.Meta,
		}
	case *sem.PoolScan:
		return &dag.PoolScan{
			Kind:   "PoolScan",
			ID:     op.ID,
			Commit: op.Commit,
		}
	case *sem.RobotScan:
		return &dag.RobotScan{
			Kind:   "RobotScan",
			Expr:   d.expr(op.Expr),
			Format: op.Format,
		}
	//
	// Ops in alphabetical order
	//
	case *sem.AggregateOp:
		return &dag.AggregateOp{
			Kind:  "AggregateOp",
			Limit: op.Limit,
			Keys:  d.assignments(op.Keys),
			Aggs:  d.assignments(op.Aggs),
		}
	case *sem.CutOp:
		return &dag.CutOp{
			Kind: "CutOp",
			Args: d.assignments(op.Args),
		}
	case *sem.DistinctOp:
		return &dag.DistinctOp{
			Kind: "DistinctOp",
			Expr: d.expr(op.Expr),
		}
	case *sem.DropOp:
		return &dag.DropOp{
			Kind: "DropOp",
			Args: d.exprs(op.Args),
		}
	case *sem.ExplodeOp:
		return &dag.ExplodeOp{
			Kind: "ExplodeOp",
			Args: d.exprs(op.Args),
			Type: op.Type,
			As:   op.As,
		}
	case *sem.FilterOp:
		return &dag.FilterOp{
			Kind: "FilterOp",
			Expr: d.expr(op.Expr),
		}
	case *sem.ForkOp:
		return &dag.ForkOp{
			Kind:  "ForkOp",
			Paths: d.paths(op.Paths),
		}
	case *sem.FuseOp:
		return &dag.FuseOp{
			Kind: "FuseOp",
		}
	case *sem.HeadOp:
		return &dag.HeadOp{
			Kind:  "HeadOp",
			Count: op.Count,
		}
	case *sem.JoinOp:
		return &dag.JoinOp{
			Kind:       "JoinOp",
			Style:      op.Style,
			LeftAlias:  op.LeftAlias,
			RightAlias: op.RightAlias,
			Cond:       d.expr(op.Cond),
		}
	case *sem.LoadOp:
		return &dag.LoadOp{
			Kind:    "LoadOp",
			Pool:    op.Pool,
			Branch:  op.Branch,
			Author:  op.Author,
			Message: op.Message,
			Meta:    op.Meta,
		}
	case *sem.MergeOp:
		if len(op.Exprs) == 0 {
			return &dag.CombineOp{Kind: "CombineOp"}
		}
		return &dag.MergeOp{
			Kind:  "MergeOp",
			Exprs: d.sortExprs(op.Exprs),
		}
	case *sem.OutputOp:
		return &dag.OutputOp{
			Kind: "OutputOp",
			Name: op.Name,
		}
	case *sem.PassOp:
		return &dag.PassOp{
			Kind: "PassOp",
		}
	case *sem.PutOp:
		return &dag.PutOp{
			Kind: "PutOp",
			Args: d.assignments(op.Args),
		}
	case *sem.RenameOp:
		return &dag.RenameOp{
			Kind: "RenameOp",
			Args: d.assignments(op.Args),
		}
	case *sem.SkipOp:
		return &dag.SkipOp{
			Kind:  "SkipOp",
			Count: op.Count,
		}
	case *sem.SortOp:
		return &dag.SortOp{
			Kind:    "SortOp",
			Exprs:   d.sortExprs(op.Exprs),
			Reverse: op.Reverse,
		}
	case *sem.SwitchOp:
		return &dag.SwitchOp{
			Kind:  "SwitchOp",
			Expr:  d.expr(op.Expr),
			Cases: d.cases(op.Cases),
		}
	case *sem.TailOp:
		return &dag.TailOp{
			Kind:  "TailOp",
			Count: op.Count,
		}
	case *sem.TopOp:
		return &dag.TopOp{
			Kind:    "TopOp",
			Limit:   op.Limit,
			Exprs:   d.sortExprs(op.Exprs),
			Reverse: op.Reverse,
		}
	case *sem.UniqOp:
		return &dag.UniqOp{
			Kind:  "UniqOp",
			Cflag: op.Cflag,
		}
	case *sem.UnnestOp:
		return &dag.UnnestOp{
			Kind: "UnnestOp",
			Expr: d.expr(op.Expr),
			Body: d.seq(op.Body),
		}
	case *sem.ValuesOp:
		return &dag.ValuesOp{
			Kind:  "ValuesOp",
			Exprs: d.exprs(op.Exprs),
		}
	}
	panic(op)
}

func (d *dagen) paths(paths []sem.Seq) []dag.Seq {
	var out []dag.Seq
	for _, path := range paths {
		out = append(out, d.seq(path))
	}
	return out
}

func (d *dagen) cases(cases []sem.Case) []dag.Case {
	var out []dag.Case
	for _, c := range cases {
		out = append(out, dag.Case{Expr: d.expr(c.Expr), Path: d.seq(c.Path)})
	}
	return out
}

func (d *dagen) assignments(assignments []sem.Assignment) []dag.Assignment {
	var out []dag.Assignment
	for _, e := range assignments {
		out = append(out, dag.Assignment{
			Kind: "Assignment",
			LHS:  d.expr(e.LHS),
			RHS:  d.expr(e.RHS),
		})
	}
	return out
}

func (d *dagen) sortExprs(exprs []sem.SortExpr) []dag.SortExpr {
	var out []dag.SortExpr
	for _, e := range exprs {
		sortExpr := dag.SortExpr{
			Key:   d.expr(e.Expr),
			Order: e.Order,
			Nulls: e.Nulls,
		}
		out = append(out, sortExpr)
	}
	return out
}

func (d *dagen) exprs(exprs []sem.Expr) []dag.Expr {
	var out []dag.Expr
	for _, e := range exprs {
		out = append(out, d.expr(e))
	}
	return out
}

func (d *dagen) expr(e sem.Expr) dag.Expr {
	switch e := e.(type) {
	case nil:
		return nil
	case *sem.AggFunc:
		return &dag.AggExpr{
			Kind:     "AggExpr",
			Name:     e.Name,
			Distinct: e.Distinct,
			Expr:     d.expr(e.Expr),
			Where:    d.expr(e.Where),
		}
	case *sem.ArrayExpr:
		return &dag.ArrayExpr{
			Kind:  "ArrayExpr",
			Elems: d.arrayElems(e.Elems),
		}
	case *sem.BinaryExpr:
		return &dag.BinaryExpr{
			Kind: "BinaryExpr",
			Op:   e.Op,
			LHS:  d.expr(e.LHS),
			RHS:  d.expr(e.RHS),
		}
	case *sem.CallExpr:
		return d.call(e)
	case *sem.CondExpr:
		return &dag.CondExpr{
			Kind: "CondExpr",
			Cond: d.expr(e.Cond),
			Then: d.expr(e.Then),
			Else: d.expr(e.Else),
		}
	case *sem.DotExpr:
		return &dag.DotExpr{
			Kind: "DotExpr",
			LHS:  d.expr(e.LHS),
			RHS:  e.RHS,
		}
	case *sem.IndexExpr:
		return &dag.IndexExpr{
			Kind:  "IndexExpr",
			Expr:  d.expr(e.Expr),
			Index: d.expr(e.Index),
		}
	case *sem.IsNullExpr:
		return &dag.IsNullExpr{
			Kind: "IsNullExpr",
			Expr: d.expr(e.Expr),
		}
	case *sem.LiteralExpr:
		return &dag.LiteralExpr{
			Kind:  "LiteralExpr",
			Value: e.Value,
		}
	case *sem.MapCallExpr:
		return &dag.MapCallExpr{
			Kind:   "MapCallExpr",
			Expr:   d.expr(e.Expr),
			Lambda: d.call(e.Lambda),
		}
	case *sem.MapExpr:
		return &dag.MapExpr{
			Kind:    "MapExpr",
			Entries: d.entries(e.Entries),
		}
	case *sem.RecordExpr:
		return &dag.RecordExpr{
			Kind:  "RecordExpr",
			Elems: d.recordElems(e.Elems),
		}
	case *sem.RegexpMatchExpr:
		return &dag.RegexpMatchExpr{
			Kind:    "RegexpMatchExpr",
			Pattern: e.Pattern,
			Expr:    d.expr(e.Expr),
		}
	case *sem.RegexpSearchExpr:
		return &dag.RegexpSearchExpr{
			Kind:    "RegexpSearchExpr",
			Pattern: e.Pattern,
			Expr:    d.expr(e.Expr),
		}
	case *sem.SearchTermExpr:
		return &dag.SearchExpr{
			Kind:  "SearchExpr",
			Text:  e.Text,
			Value: e.Value,
			Expr:  d.expr(e.Expr),
		}
	case *sem.SetExpr:
		return &dag.SetExpr{
			Kind:  "SetExpr",
			Elems: d.arrayElems(e.Elems),
		}
	case *sem.SliceExpr:
		return &dag.SliceExpr{
			Kind: "SliceExpr",
			Expr: d.expr(e.Expr),
			From: d.expr(e.From),
			To:   d.expr(e.To),
		}
	case *sem.SubqueryExpr:
		return d.subquery(e)
	case *sem.ThisExpr:
		return &dag.ThisExpr{
			Kind: "ThisExpr",
			Path: e.Path,
		}
	case *sem.UnaryExpr:
		return &dag.UnaryExpr{
			Kind:    "UnaryExpr",
			Op:      e.Op,
			Operand: d.expr(e.Operand),
		}
	}
	panic(e)
}

func (d *dagen) arrayElems(elems []sem.ArrayElem) []dag.VectorElem {
	var out []dag.VectorElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *sem.SpreadElem:
			out = append(out, d.spread(elem.Expr))
		case *sem.ExprElem:
			out = append(out, &dag.VectorValue{Kind: "VectorValue", Expr: d.expr(elem.Expr)})
		default:
			panic(elem)
		}
	}
	return out
}

func (d *dagen) recordElems(elems []sem.RecordElem) []dag.RecordElem {
	var out []dag.RecordElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *sem.SpreadElem:
			out = append(out, d.spread(elem.Expr))
		case *sem.FieldElem:
			out = append(out, &dag.Field{Kind: "Field", Name: elem.Name, Value: d.expr(elem.Value)})
		default:
			panic(elem)
		}
	}
	return out
}

func (d *dagen) spread(e sem.Expr) *dag.Spread {
	return &dag.Spread{
		Kind: "Spread",
		Expr: d.expr(e),
	}
}

func (d *dagen) entries(entries []sem.Entry) []dag.Entry {
	var out []dag.Entry
	for _, entry := range entries {
		out = append(out, dag.Entry{Key: d.expr(entry.Key), Value: d.expr(entry.Value)})
	}
	return out
}

func (d *dagen) subquery(e *sem.SubqueryExpr) *dag.SubqueryExpr {
	subquery := &dag.SubqueryExpr{
		Kind:       "SubqueryExpr",
		Correlated: e.Correlated,
		Body:       d.seq(e.Body),
	}
	if e.Array {
		subquery.Body = collectThis(subquery.Body)
	}
	return subquery
}

func collectThis(seq dag.Seq) dag.Seq {
	collect := dag.Assignment{
		Kind: "Assignment",
		LHS:  dag.NewThis([]string{"collect"}),
		RHS:  &dag.AggExpr{Kind: "AggExpr", Name: "collect", Expr: dag.NewThis(nil)},
	}
	aggOp := &dag.AggregateOp{
		Kind: "AggregateOp",
		Aggs: []dag.Assignment{collect},
	}
	emitOp := &dag.ValuesOp{
		Kind:  "ValuesOp",
		Exprs: []dag.Expr{dag.NewThis([]string{"collect"})},
	}
	seq = append(seq, aggOp)
	return append(seq, emitOp)
}

func (d *dagen) call(c *sem.CallExpr) *dag.CallExpr {
	return &dag.CallExpr{
		Kind: "CallExpr",
		Tag:  c.Tag,
		Args: d.exprs(c.Args),
	}
}

func (d *dagen) fn(f *funcDef) *dag.FuncDef {
	return &dag.FuncDef{
		Kind:   "FuncDef",
		Tag:    f.tag,
		Name:   f.name,
		Params: f.params,
		Expr:   d.expr(f.body),
	}
}

func (d *dagen) debugOp(o *sem.DebugOp, branch sem.Seq, seq dag.Seq) dag.Seq {
	output := &dag.OutputOp{Kind: "OutputOp", Name: "debug"}
	d.outputs[output] = o
	e := d.expr(o.Expr)
	if e == nil {
		e = dag.NewThis(nil)
	}
	y := &dag.ValuesOp{Kind: "ValuesOp", Exprs: []dag.Expr{e}}
	main := d.seq(branch)
	if len(main) == 0 {
		main.Append(&dag.PassOp{Kind: "PassOp"})
	}
	return append(seq, &dag.MirrorOp{
		Kind:   "MirrorOp",
		Main:   main,
		Mirror: dag.Seq{y, output},
	})
}

func (d *dagen) checkOutputs(isLeaf bool, seq dag.Seq) dag.Seq {
	if len(seq) == 0 {
		return seq
	}
	// - Report an error in any outputs are not located in the leaves.
	// - Add output operators to any leaves where they do not exist.
	lastN := len(seq) - 1
	for i, o := range seq {
		isLast := lastN == i
		switch o := o.(type) {
		case *dag.OutputOp:
			if !isLast || !isLeaf {
				n, ok := d.outputs[o]
				if !ok {
					panic("system error: untracked user output")
				}
				d.error(n, errors.New("output operator must be at flowgraph leaf"))
			}
		case *dag.ScatterOp:
			for k := range o.Paths {
				o.Paths[k] = d.checkOutputs(isLast && isLeaf, o.Paths[k])
			}
		case *dag.UnnestOp:
			o.Body = d.checkOutputs(false, o.Body)
		case *dag.ForkOp:
			for k := range o.Paths {
				o.Paths[k] = d.checkOutputs(isLast && isLeaf, o.Paths[k])
			}
		case *dag.SwitchOp:
			for k := range o.Cases {
				o.Cases[k].Path = d.checkOutputs(isLast && isLeaf, o.Cases[k].Path)
			}
		case *dag.MirrorOp:
			o.Main = d.checkOutputs(isLast && isLeaf, o.Main)
			o.Mirror = d.checkOutputs(isLast && isLeaf, o.Mirror)
		}
	}
	switch seq[lastN].(type) {
	case *dag.ForkOp, *dag.MirrorOp, *dag.OutputOp, *dag.ScatterOp, *dag.SwitchOp:
	default:
		if isLeaf {
			return append(seq, &dag.OutputOp{Kind: "OutputOp", Name: "main"})
		}
	}
	return seq
}
