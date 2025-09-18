package semantic

import (
	"fmt"
	"strconv"

	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/runtime/sam/expr/function"
)

type resolver struct {
	analyzer *analyzer
	in       map[string]*dag.FuncDef
	fixed    map[string]*dag.FuncDef
	variants []*dag.FuncDef
	params   []map[string]string
	ntag     int
}

func newResolver(a *analyzer) *resolver {
	r := &resolver{
		analyzer: a,
		in:       a.funcsByTag,
		fixed:    make(map[string]*dag.FuncDef),
	}
	return r
}

func (r *resolver) resolve(seq dag.Seq) (dag.Seq, []*dag.FuncDef) {
	out := r.resolveSeq(seq)
	funcs := r.variants
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
// removed from the DAG.  All functions are converted from the input function table
// to the output function table whether or not they needed to be resolved.
// Any FuncRefs that do not get bound to a CallParam (i.e., appear in random expressions)
// are found and reported as error as are any CallParam that are called with non-FuncRef
// arguments.
func (r *resolver) resolveSeq(seq dag.Seq) dag.Seq {
	var out dag.Seq
	for _, op := range seq {
		out = append(out, r.resolveOp(op))
	}
	return out
}

func (r *resolver) resolveOp(op dag.Op) dag.Op {
	switch op := op.(type) {
	case *dag.Aggregate:
		return &dag.Aggregate{
			Kind:         "Aggregate",
			Limit:        op.Limit,
			Keys:         r.resolveAssignments(op.Keys),
			Aggs:         r.resolveAssignments(op.Aggs),
			InputSortDir: op.InputSortDir,
			PartialsIn:   op.PartialsIn,
			PartialsOut:  op.PartialsOut,
		}
	case *dag.BadOp:
	case *dag.Fork:
		var paths []dag.Seq
		for _, seq := range op.Paths {
			paths = append(paths, r.resolveSeq(seq))
		}
		return &dag.Fork{
			Kind:  "Fork",
			Paths: paths,
		}
	case *dag.Scatter:
		var paths []dag.Seq
		for _, seq := range op.Paths {
			paths = append(paths, r.resolveSeq(seq))
		}
		return &dag.Fork{
			Kind:  "Fork",
			Paths: paths,
		}
	case *dag.Switch:
		var cases []dag.Case
		for _, c := range op.Cases {
			cases = append(cases, dag.Case{
				Expr: r.resolveExpr(c.Expr),
				Path: r.resolveSeq(c.Path),
			})
		}
		return &dag.Switch{
			Kind:  "Switch",
			Expr:  r.resolveExpr(op.Expr),
			Cases: cases,
		}
	case *dag.Sort:
		return &dag.Sort{
			Kind:    "Sort",
			Exprs:   r.resolveSortExprs(op.Exprs),
			Reverse: op.Reverse,
		}
	case *dag.Cut:
		return &dag.Cut{
			Kind: "Cut",
			Args: r.resolveAssignments(op.Args),
		}
	case *dag.Distinct:
		return &dag.Distinct{
			Kind: "Distinct",
			Expr: r.resolveExpr(op.Expr),
		}
	case *dag.Drop:
		return &dag.Drop{
			Kind: "Drop",
			Args: r.resolveExprs(op.Args),
		}
	case *dag.Head:
	case *dag.Tail:
	case *dag.Skip:
	case *dag.Pass:
	case *dag.Filter:
		return &dag.Filter{
			Kind: "Filter",
			Expr: r.resolveExpr(op.Expr),
		}
	case *dag.Uniq:
	case *dag.Top:
		return &dag.Top{
			Kind:    "Top",
			Limit:   op.Limit,
			Exprs:   r.resolveSortExprs(op.Exprs),
			Reverse: op.Reverse,
		}
	case *dag.Put:
		return &dag.Put{
			Kind: "Put",
			Args: r.resolveAssignments(op.Args),
		}
	case *dag.Rename:
		return &dag.Rename{
			Kind: "Rename",
			Args: r.resolveAssignments(op.Args),
		}
	case *dag.Fuse:
	case *dag.HashJoin:
		return &dag.HashJoin{
			Kind:       "HashJoin",
			Style:      op.Style,
			LeftAlias:  op.LeftAlias,
			RightAlias: op.RightAlias,
			LeftKey:    r.resolveExpr(op.LeftKey),
			RightKey:   r.resolveExpr(op.RightKey),
		}
	case *dag.Join:
		return &dag.Join{
			Kind:       "Join",
			Style:      op.Style,
			LeftAlias:  op.LeftAlias,
			RightAlias: op.RightAlias,
			Cond:       r.resolveExpr(op.Cond),
		}
	case *dag.Explode:
		return &dag.Explode{
			Kind: "Explode",
			Args: r.resolveExprs(op.Args),
			Type: op.Type,
			As:   op.As,
		}
	case *dag.Unnest:
		return &dag.Unnest{
			Kind: "Unnest",
			Expr: r.resolveExpr(op.Expr),
			Body: r.resolveSeq(op.Body),
		}
	case *dag.Values:
		return &dag.Values{
			Kind:  "Values",
			Exprs: r.resolveExprs(op.Exprs),
		}
	case *dag.Merge:
		return &dag.Merge{
			Kind:  "Merge",
			Exprs: r.resolveSortExprs(op.Exprs),
		}
	case *dag.Mirror:
		return &dag.Mirror{
			Kind:   "Mirror",
			Main:   r.resolveSeq(op.Main),
			Mirror: r.resolveSeq(op.Mirror),
		}
	case *dag.Combine:
	case *dag.Load:
	case *dag.Output:
	case *dag.DefaultScan:
	case *dag.FileScan:
	case *dag.HTTPScan:
	case *dag.PoolScan:
	case *dag.RobotScan:
		return &dag.RobotScan{
			Kind:   "RobotScan",
			Expr:   r.resolveExpr(op.Expr),
			Format: op.Format,
			Filter: r.resolveExpr(op.Filter),
		}
	case *dag.DeleteScan:
	case *dag.DBMetaScan:
	case *dag.PoolMetaScan:
	case *dag.CommitMetaScan:
	case *dag.NullScan:
	case *dag.Lister:
	case *dag.Slicer:
	case *dag.SeqScan:
	case *dag.Deleter:
		return &dag.Deleter{
			Kind:      "Deleter",
			Pool:      op.Pool,
			Where:     r.resolveExpr(op.Where),
			KeyPruner: r.resolveExpr(op.KeyPruner),
		}
	default:
		panic(op)
	}
	return op
}

func (r *resolver) resolveAssignments(assignments []dag.Assignment) []dag.Assignment {
	var out []dag.Assignment
	for _, ass := range assignments {
		out = append(out, dag.Assignment{
			Kind: "Assignment",
			LHS:  r.resolveExpr(ass.LHS),
			RHS:  r.resolveExpr(ass.RHS),
		})
	}
	return out
}

func (r *resolver) resolveSortExprs(exprs []dag.SortExpr) []dag.SortExpr {
	var out []dag.SortExpr
	for _, e := range exprs {
		out = append(out, dag.SortExpr{
			Key:   r.resolveExpr(e.Key),
			Order: e.Order,
			Nulls: e.Nulls})
	}
	return out
}

func (r *resolver) resolveExprs(exprs []dag.Expr) []dag.Expr {
	var out []dag.Expr
	for _, e := range exprs {
		out = append(out, r.resolveExpr(e))
	}
	return out
}

func (r *resolver) resolveExpr(e dag.Expr) dag.Expr {
	switch e := e.(type) {
	case nil:
		return nil
	case *dag.FuncRef:
		// This needs to be in an argument list and can't be anywhere else... bad DAG
		panic(e)
	case *dag.CallParam:
		// This is a call to a parameter.  It must only appear enclosed in a FuncDef
		// with params containing the name in the e.Param.  The function being referenced
		// or passed in can be lazily created.
		return r.resolveCallParam(e)
	case *dag.Agg:
		return &dag.Agg{
			Kind:     "Agg",
			Name:     e.Name,
			Distinct: e.Distinct,
			Expr:     r.resolveExpr(e.Expr),
			Where:    r.resolveExpr(e.Where),
		}
	case *dag.ArrayExpr:
		return &dag.ArrayExpr{
			Kind:  "ArrayExpr",
			Elems: r.resolveVectorElems(e.Elems),
		}
	case *dag.BadExpr:
	case *dag.BinaryExpr:
		return dag.NewBinaryExpr(e.Op, r.resolveExpr(e.LHS), r.resolveExpr(e.RHS))
	case *dag.Call:
		return r.resolveCall(r.analyzer.locs[e], e.Tag, e.Args)
	case *dag.Conditional:
		return &dag.Conditional{
			Kind: "Conditional",
			Cond: r.resolveExpr(e.Cond),
			Then: r.resolveExpr(e.Then),
			Else: r.resolveExpr(e.Else),
		}
	case *dag.Dot:
		return &dag.Dot{
			Kind: "Dot",
			LHS:  r.resolveExpr(e.LHS),
			RHS:  e.RHS,
		}
	case *dag.IndexExpr:
		return &dag.IndexExpr{
			Kind:  "IndexExpr",
			Expr:  r.resolveExpr(e.Expr),
			Index: r.resolveExpr(e.Index),
		}
	case *dag.IsNullExpr:
		return &dag.IsNullExpr{
			Kind: "IsNullExpr",
			Expr: r.resolveExpr(e.Expr),
		}
	case *dag.Literal:
	case *dag.MapCall:
		loc := r.analyzer.locs[e]
		call, ok := r.resolveCall(loc, e.Lambda.Tag, e.Lambda.Args).(*dag.Call)
		if !ok {
			return badExpr()
		}
		return &dag.MapCall{
			Kind:   "MapCall",
			Expr:   r.resolveExpr(e.Expr),
			Lambda: call,
		}
	case *dag.MapExpr:
		var entries []dag.Entry
		for _, entry := range e.Entries {
			entries = append(entries, dag.Entry{
				Key:   r.resolveExpr(entry.Key),
				Value: r.resolveExpr(entry.Value),
			})
		}
		return &dag.MapExpr{
			Kind:    "MapExpr",
			Entries: entries,
		}
	case *dag.RecordExpr:
		return &dag.RecordExpr{
			Kind:  "RecordExpr",
			Elems: r.resolveRecordElems(e.Elems),
		}
	case *dag.RegexpMatch:
		return &dag.RegexpMatch{
			Kind:    "RegexpMatch",
			Pattern: e.Pattern,
			Expr:    r.resolveExpr(e.Expr),
		}
	case *dag.RegexpSearch:
		return &dag.RegexpSearch{
			Kind:    "RegexpSearch",
			Pattern: e.Pattern,
			Expr:    r.resolveExpr(e.Expr),
		}
	case *dag.Search:
		return &dag.Search{
			Kind:  "Search",
			Text:  e.Text,
			Value: e.Value,
			Expr:  r.resolveExpr(e.Expr),
		}
	case *dag.SetExpr:
		return &dag.SetExpr{
			Kind:  "SetExpr",
			Elems: r.resolveVectorElems(e.Elems),
		}
	case *dag.SliceExpr:
		return &dag.SliceExpr{
			Kind: "SliceExpr",
			Expr: r.resolveExpr(e.Expr),
			From: r.resolveExpr(e.From),
			To:   r.resolveExpr(e.To),
		}
	case *dag.Subquery:
		// We clear params before processing a subquery so you can't
		// touch passed-in functions inside the first "this" of a correlated
		// subquery.  We can support this later if people are interested.
		// It requires a bit of surgery.
		r.pushParams(make(map[string]string))
		defer r.popParams()
		return &dag.Subquery{
			Kind:       "Subquery",
			Correlated: e.Correlated,
			Body:       r.resolveSeq(e.Body),
		}
	case *dag.This:
	case *dag.UnaryExpr:
		return dag.NewUnaryExpr(e.Op, r.resolveExpr(e.Operand))
	default:
		panic(e)
	}
	return e
}

func (r *resolver) resolveVectorElems(elems []dag.VectorElem) []dag.VectorElem {
	var out []dag.VectorElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *dag.Spread:
			out = append(out, &dag.Spread{
				Kind: "Spread",
				Expr: r.resolveExpr(elem.Expr),
			})
		case *dag.VectorValue:
			out = append(out, &dag.VectorValue{
				Kind: "VectorValue",
				Expr: r.resolveExpr(elem.Expr),
			})
		default:
			panic(elem)
		}
	}
	return out
}

func (r *resolver) resolveRecordElems(elems []dag.RecordElem) []dag.RecordElem {
	var out []dag.RecordElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *dag.Spread:
			out = append(out, &dag.Spread{
				Kind: "Spread",
				Expr: r.resolveExpr(elem.Expr),
			})
		case *dag.Field:
			out = append(out, &dag.Field{
				Kind:  "Field",
				Name:  elem.Name,
				Value: r.resolveExpr(elem.Value),
			})
		default:
			panic(elem)
		}
	}
	return out
}

func (r *resolver) error(loc ast.Loc, err error) {
	r.analyzer.error(loc, err)
}

func (r *resolver) resolveCallParam(call *dag.CallParam) dag.Expr {
	oldTag := r.lookupParam(call.Param)
	if oldTag == "" {
		// This can happen when we go to resolve a parameter that wasn't bound to
		// an actual function because some other value was bound to it so it didn't
		// get put in the parameter table.
		r.error(r.analyzer.locs[call], fmt.Errorf("function called via parameter %q is bound to a non-function", call.Param))
		return badExpr()
	}
	if isBuiltin(oldTag) {
		// Check argument count here for builtin functions.
		if _, err := function.New(r.analyzer.sctx, oldTag, len(call.Args)); err != nil {
			r.error(r.analyzer.locs[call], fmt.Errorf("function %q called via parameter %q: %w", oldTag, call.Param, err))
			return badExpr()
		}
	}
	return r.resolveCall(r.analyzer.locs[call], oldTag, call.Args)
}

func (r *resolver) resolveCall(callLoc ast.Loc, oldTag string, args []dag.Expr) dag.Expr {
	if isBuiltin(oldTag) {
		return &dag.Call{
			Kind: "Call",
			Tag:  oldTag,
			Args: r.resolveExprs(args),
		}
	}
	// Translate the tag to the new func table and convert any
	// function refs passed as args to lookup-table removing
	// correponding args.
	var params []string
	var exprs []dag.Expr
	funcDef := r.in[oldTag]
	if len(funcDef.Params) != len(args) {
		r.error(callLoc, fmt.Errorf("%q: expected %d params but called with %d", funcDef.Name, len(funcDef.Params), len(args)))
		return badExpr()
	}
	bindings := make(map[string]string)
	for k, arg := range args {
		if f, ok := arg.(*dag.FuncRef); ok {
			bindings[funcDef.Params[k]] = f.Tag
			continue
		}
		e := r.resolveExpr(arg)
		if e, ok := e.(*dag.This); ok {
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
		exprs = append(exprs, r.resolveExpr(arg))
	}
	if len(funcDef.Params) == len(params) {
		// No need to specialize this call since no function args are being passed.
		newTag := r.lookupFixed(oldTag)
		return &dag.Call{
			Kind: "Call",
			Tag:  newTag,
			Args: args,
		}
	}
	// Enter the new function scope and set up the bindings for the
	// values we retrieved above while evaluating args in the outer scope.
	r.pushParams(bindings)
	defer r.popParams()
	newTag := r.lookupVariant(oldTag, params)
	return &dag.Call{
		Kind: "Call",
		Tag:  newTag,
		Args: exprs,
	}
}

func (r *resolver) lookupFixed(oldTag string) string {
	if funcDef, ok := r.fixed[oldTag]; ok {
		return funcDef.Tag
	}
	funcDef := r.in[oldTag]
	newTag := r.nextTag()
	newFuncDef := &dag.FuncDef{
		Kind:   "FuncDef",
		Tag:    newTag,
		Name:   funcDef.Name,
		Params: funcDef.Params,
	}
	r.fixed[oldTag] = newFuncDef
	newFuncDef.Expr = r.resolveExpr(funcDef.Expr)
	return newTag
}

func (r *resolver) lookupVariant(oldTag string, params []string) string {
	newTag := r.nextTag()
	funcDef := r.in[oldTag]
	r.variants = append(r.variants, &dag.FuncDef{
		Kind:   "FuncDef",
		Tag:    newTag,
		Name:   funcDef.Name,
		Params: params,
		Expr:   r.resolveExpr(funcDef.Expr), // since params have been bound this will convert the CallParams
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
