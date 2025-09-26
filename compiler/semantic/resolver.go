package semantic

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/runtime/sam/expr/function"
)

type resolver struct {
	t        *translator
	fixed    map[string]*sem.FuncDef
	variants []*sem.FuncDef
	params   []map[string]string
	ntag     int
}

func newResolver(t *translator) *resolver {
	return &resolver{
		t:     t,
		fixed: make(map[string]*sem.FuncDef),
	}
}

func (r *resolver) resolve(seq sem.Seq) (sem.Seq, []*sem.FuncDef) {
	out := r.seq(seq)
	funcs := slices.Clone(r.variants)
	for _, f := range r.fixed {
		funcs = append(funcs, f)
	}
	return out, funcs
}

func (r *resolver) resolveExpr(e sem.Expr) (sem.Expr, []*sem.FuncDef) {
	out := r.expr(e)
	funcs := slices.Clone(r.variants)
	for _, f := range r.fixed {
		funcs = append(funcs, f)
	}
	return out, funcs
}

// resolveSeq traverses a DAG and substitues all instances of CallParam by rewriting
// each called function that incudes one or more CallParams substituting the actual
// function called (which is also possibly resolved) as indicated by a FuncRef.
// Each function is also rewritten to remove the function parameters from its params.
// After resolve is done, all CallParam or FuncRef pseudo-expression nodes are
// removed from the sem.  All functions are converted from the input function table
// to the output function table whether or not they needed to be resolved.
// Any FuncRefs that do not get bound to a CallParam (i.e., appear in random expressions)
// are found and reported as error as are any CallParam that are called with non-FuncRef
// arguments.
func (r *resolver) seq(seq sem.Seq) sem.Seq {
	var out sem.Seq
	for _, op := range seq {
		out = append(out, r.op(op))
	}
	return out
}

func (r *resolver) op(op sem.Op) sem.Op {
	switch op := op.(type) {
	//
	// Scanners first
	//
	case *sem.CommitMetaScan:
	case *sem.DBMetaScan:
	case *sem.DefaultScan:
	case *sem.DeleteScan:
	case *sem.FileScan:
	case *sem.HTTPScan:
	case *sem.NullScan:
	case *sem.PoolMetaScan:
	case *sem.PoolScan:
	case *sem.RobotScan:
		return &sem.RobotScan{
			Node:   op.Node,
			Expr:   r.expr(op.Expr),
			Format: op.Format,
		}
	//
	// Ops in alphabetic order
	//
	case *sem.AggregateOp:
		return &sem.AggregateOp{
			Node:  op.Node,
			Limit: op.Limit,
			Keys:  r.assignments(op.Keys),
			Aggs:  r.assignments(op.Aggs),
		}
	case *sem.BadOp:
	case *sem.CutOp:
		return &sem.CutOp{
			Node: op.Node,
			Args: r.assignments(op.Args),
		}
	case *sem.DebugOp:
		return &sem.DebugOp{
			Node: op.Node,
			Expr: r.expr(op.Expr),
		}
	case *sem.DistinctOp:
		return &sem.DistinctOp{
			Node: op.Node,
			Expr: r.expr(op.Expr),
		}
	case *sem.DropOp:
		return &sem.DropOp{
			Node: op.Node,
			Args: r.exprs(op.Args),
		}
	case *sem.ExplodeOp:
		return &sem.ExplodeOp{
			Node: op.Node,
			Args: r.exprs(op.Args),
			Type: op.Type,
			As:   op.As,
		}
	case *sem.FilterOp:
		return &sem.FilterOp{
			Node: op.Node,
			Expr: r.expr(op.Expr),
		}
	case *sem.ForkOp:
		var paths []sem.Seq
		for _, seq := range op.Paths {
			paths = append(paths, r.seq(seq))
		}
		return &sem.ForkOp{
			Node:  op.Node,
			Paths: paths,
		}
	case *sem.FuseOp:
	case *sem.HeadOp:
	case *sem.JoinOp:
		return &sem.JoinOp{
			Node:       op.Node,
			Style:      op.Style,
			LeftAlias:  op.LeftAlias,
			RightAlias: op.RightAlias,
			Cond:       r.expr(op.Cond),
		}
	case *sem.LoadOp:
	case *sem.MergeOp:
		return &sem.MergeOp{
			Node:  op.Node,
			Exprs: r.sortExprs(op.Exprs),
		}
	case *sem.OutputOp:
	case *sem.PassOp:
	case *sem.PutOp:
		return &sem.PutOp{
			Node: op.Node,
			Args: r.assignments(op.Args),
		}
	case *sem.RenameOp:
		return &sem.RenameOp{
			Node: op.Node,
			Args: r.assignments(op.Args),
		}
	case *sem.SkipOp:
	case *sem.SortOp:
		return &sem.SortOp{
			Node:    op.Node,
			Exprs:   r.sortExprs(op.Exprs),
			Reverse: op.Reverse,
		}
	case *sem.SwitchOp:
		var cases []sem.Case
		for _, c := range op.Cases {
			cases = append(cases, sem.Case{
				Expr: r.expr(c.Expr),
				Path: r.seq(c.Path),
			})
		}
		return &sem.SwitchOp{
			Node:  op.Node,
			Expr:  r.expr(op.Expr),
			Cases: cases,
		}
	case *sem.TailOp:
	case *sem.TopOp:
		return &sem.TopOp{
			Node:  op.Node,
			Limit: op.Limit,
			Exprs: r.sortExprs(op.Exprs),
		}
	case *sem.UniqOp:
	case *sem.UnnestOp:
		return &sem.UnnestOp{
			Node: op.Node,
			Expr: r.expr(op.Expr),
			Body: r.seq(op.Body),
		}
	case *sem.ValuesOp:
		return &sem.ValuesOp{
			Node:  op.Node,
			Exprs: r.exprs(op.Exprs),
		}

	default:
		panic(op)
	}
	return op
}

func (r *resolver) assignments(assignments []sem.Assignment) []sem.Assignment {
	var out []sem.Assignment
	for _, a := range assignments {
		out = append(out, sem.Assignment{
			Node: a.Node,
			LHS:  r.expr(a.LHS),
			RHS:  r.expr(a.RHS),
		})
	}
	return out
}

func (r *resolver) sortExprs(exprs []sem.SortExpr) []sem.SortExpr {
	var out []sem.SortExpr
	for _, e := range exprs {
		out = append(out, sem.SortExpr{
			Node:  e.Node,
			Expr:  r.expr(e.Expr),
			Order: e.Order,
			Nulls: e.Nulls})
	}
	return out
}

func (r *resolver) exprs(exprs []sem.Expr) []sem.Expr {
	var out []sem.Expr
	for _, e := range exprs {
		out = append(out, r.expr(e))
	}
	return out
}

func (r *resolver) expr(e sem.Expr) sem.Expr {
	switch e := e.(type) {
	case nil:
		return nil
	case *sem.AggFunc:
		return &sem.AggFunc{
			Node:     e.Node,
			Name:     e.Name,
			Distinct: e.Distinct,
			Expr:     r.expr(e.Expr),
			Where:    r.expr(e.Where),
		}
	case *sem.ArrayExpr:
		return &sem.ArrayExpr{
			Node:  e.Node,
			Elems: r.arrayElems(e.Elems),
		}
	case *sem.BadExpr:
	case *sem.BinaryExpr:
		return sem.NewBinaryExpr(e.Node, e.Op, r.expr(e.LHS), r.expr(e.RHS))
	case *sem.CallExpr:
		return r.resolveCall(e.Node, e.Tag, e.Args)
	case *sem.CallParam:
		// This is a call to a parameter.  It must only appear enclosed in a FuncDef
		// with params containing the name in the e.Param.  The function being referenced
		// or passed in can be lazily created.
		return r.resolveCallParam(e)
	case *sem.CondExpr:
		return &sem.CondExpr{
			Node: e.Node,
			Cond: r.expr(e.Cond),
			Then: r.expr(e.Then),
			Else: r.expr(e.Else),
		}
	case *sem.DotExpr:
		return &sem.DotExpr{
			Node: e.Node,
			LHS:  r.expr(e.LHS),
			RHS:  e.RHS,
		}
	case *sem.FuncRef:
		// This needs to be in an argument list and can't be anywhere else... bad DAG
		panic(e)
	case *sem.IndexExpr:
		return &sem.IndexExpr{
			Node:  e.Node,
			Expr:  r.expr(e.Expr),
			Index: r.expr(e.Index),
		}
	case *sem.IsNullExpr:
		return &sem.IsNullExpr{
			Node: e.Node,
			Expr: r.expr(e.Expr),
		}
	case *sem.LiteralExpr:
	case *sem.MapCallExpr:
		call, ok := r.resolveCall(e.Node, e.Lambda.Tag, e.Lambda.Args).(*sem.CallExpr)
		if !ok {
			return badExpr()
		}
		return &sem.MapCallExpr{
			Node:   e.Node,
			Expr:   r.expr(e.Expr),
			Lambda: call,
		}
	case *sem.MapExpr:
		var entries []sem.Entry
		for _, entry := range e.Entries {
			entries = append(entries, sem.Entry{
				Key:   r.expr(entry.Key),
				Value: r.expr(entry.Value),
			})
		}
		return &sem.MapExpr{
			Node:    e.Node,
			Entries: entries,
		}
	case *sem.RecordExpr:
		return &sem.RecordExpr{
			Node:  e.Node,
			Elems: r.recordElems(e.Elems),
		}
	case *sem.RegexpMatchExpr:
		return &sem.RegexpMatchExpr{
			Node:    e.Node,
			Pattern: e.Pattern,
			Expr:    r.expr(e.Expr),
		}
	case *sem.RegexpSearchExpr:
		return &sem.RegexpSearchExpr{
			Node:    e.Node,
			Pattern: e.Pattern,
			Expr:    r.expr(e.Expr),
		}
	case *sem.SearchTermExpr:
		return &sem.SearchTermExpr{
			Node:  e.Node,
			Text:  e.Text,
			Value: e.Value,
			Expr:  r.expr(e.Expr),
		}
	case *sem.SetExpr:
		return &sem.SetExpr{
			Node:  e.Node,
			Elems: r.arrayElems(e.Elems),
		}
	case *sem.SliceExpr:
		return &sem.SliceExpr{
			Node: e.Node,
			Expr: r.expr(e.Expr),
			From: r.expr(e.From),
			To:   r.expr(e.To),
		}
	case *sem.SubqueryExpr:
		// We clear params before processing a subquery so you can't
		// touch passed-in functions inside the first "this" of a correlated
		// subquery.  We can support this later if people are interested.
		// It requires a bit of surgery.
		r.pushParams(make(map[string]string))
		defer r.popParams()
		return &sem.SubqueryExpr{
			Node:       e.Node,
			Correlated: e.Correlated,
			Array:      e.Array,
			Body:       r.seq(e.Body),
		}
	case *sem.ThisExpr:
	case *sem.UnaryExpr:
		return sem.NewUnaryExpr(e.Node, e.Op, r.expr(e.Operand))
	default:
		panic(e)
	}
	return e
}

func (r *resolver) arrayElems(elems []sem.ArrayElem) []sem.ArrayElem {
	var out []sem.ArrayElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *sem.SpreadElem:
			out = append(out, &sem.SpreadElem{
				Node: elem.Node,
				Expr: r.expr(elem.Expr),
			})
		case *sem.ExprElem:
			out = append(out, &sem.ExprElem{
				Node: elem.Node,
				Expr: r.expr(elem.Expr),
			})
		default:
			panic(elem)
		}
	}
	return out
}

func (r *resolver) recordElems(elems []sem.RecordElem) []sem.RecordElem {
	var out []sem.RecordElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *sem.SpreadElem:
			out = append(out, &sem.SpreadElem{
				Node: elem.Node,
				Expr: r.expr(elem.Expr),
			})
		case *sem.FieldElem:
			out = append(out, &sem.FieldElem{
				Node:  elem.Node,
				Name:  elem.Name,
				Value: r.expr(elem.Value),
			})
		default:
			panic(elem)
		}
	}
	return out
}

func (r *resolver) resolveCallParam(call *sem.CallParam) sem.Expr {
	oldTag := r.lookupParam(call.Param)
	if oldTag == "" {
		// This can happen when we go to resolve a parameter that wasn't bound to
		// an actual function because some other value was bound to it so it didn't
		// get put in the parameter table.
		r.t.error(call.Node, fmt.Errorf("function called via parameter %q is bound to a non-function", call.Param))
		return badExpr()
	}
	if isBuiltin(oldTag) {
		// Check argument count here for builtin functions.
		if _, err := function.New(super.NewContext(), oldTag, len(call.Args)); err != nil {
			r.t.error(call.Node, fmt.Errorf("function %q called via parameter %q: %w", oldTag, call.Param, err))
			return badExpr()
		}
	}
	return r.resolveCall(call.Node, oldTag, call.Args)
}

func (r *resolver) resolveCall(n ast.Node, oldTag string, args []sem.Expr) sem.Expr {
	if isBuiltin(oldTag) {
		return &sem.CallExpr{
			Node: n,
			Tag:  oldTag,
			Args: r.exprs(args),
		}
	}
	// Translate the tag to the new func table and convert any
	// function refs passed as args to lookup-table removing
	// correponding args.
	var params []string
	var exprs []sem.Expr
	funcDef := r.t.resolveFunc(oldTag)
	if len(funcDef.Params) != len(args) {
		r.t.error(n, fmt.Errorf("%q: expected %d params but called with %d", funcDef.Name, len(funcDef.Params), len(args)))
		return badExpr()
	}
	bindings := make(map[string]string)
	for k, arg := range args {
		if f, ok := arg.(*sem.FuncRef); ok {
			bindings[funcDef.Params[k]] = f.Tag
			continue
		}
		e := r.expr(arg)
		if e, ok := e.(*sem.ThisExpr); ok {
			if len(e.Path) == 1 {
				// Propagate a function passed as a function value inside of
				// a function to another function.
				if tag := r.lookupParam(e.Path[0]); tag != "" {
					bindings[funcDef.Params[k]] = tag
					continue
				}
			}
		}
		params = append(params, funcDef.Params[k])
		exprs = append(exprs, r.expr(arg))
	}
	if len(funcDef.Params) == len(params) {
		// No need to specialize this call since no function args are being passed.
		newTag := r.lookupFixed(oldTag)
		return &sem.CallExpr{
			Node: n,
			Tag:  newTag,
			Args: args,
		}
	}
	// Enter the new function scope and set up the bindings for the
	// values we retrieved above while evaluating args in the outer scope.
	r.pushParams(bindings)
	defer r.popParams()
	newTag := r.lookupVariant(oldTag, params)
	return &sem.CallExpr{
		Node: n,
		Tag:  newTag,
		Args: exprs,
	}
}

func (r *resolver) lookupFixed(oldTag string) string {
	if funcDef, ok := r.fixed[oldTag]; ok {
		return funcDef.Tag
	}
	funcDef := r.t.resolveFunc(oldTag)
	newTag := r.nextTag()
	newFuncDef := &sem.FuncDef{
		Node:   funcDef.Node,
		Tag:    newTag,
		Name:   funcDef.Name,
		Params: funcDef.Params,
	}
	r.fixed[oldTag] = newFuncDef
	newFuncDef.Body = r.expr(funcDef.Body)
	return newTag
}

func (r *resolver) lookupVariant(oldTag string, params []string) string {
	newTag := r.nextTag()
	funcDef := r.t.resolveFunc(oldTag)
	r.variants = append(r.variants, &sem.FuncDef{
		Node:   funcDef.Node,
		Tag:    newTag,
		Name:   funcDef.Name,
		Params: params,
		Body:   r.expr(funcDef.Body), // since params have been bound this will convert the CallParams
	})
	return newTag
}

func (r *resolver) nextTag() string {
	tag := strconv.Itoa(r.ntag)
	r.ntag++
	return tag
}

func (r *resolver) pushParams(scope map[string]string) {
	r.params = append(r.params, scope)
}

func (r *resolver) popParams() {
	r.params = r.params[0 : len(r.params)-1]
}

func (r *resolver) lookupParam(param string) string {
	if len(r.params) == 0 {
		return ""
	}
	return r.params[len(r.params)-1][param]
}

func isBuiltin(tag string) bool {
	_, err := strconv.Atoi(tag)
	return err != nil
}
