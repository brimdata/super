package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/brimsec/zq/ast"
	"github.com/brimsec/zq/field"
	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/proc"
	"github.com/brimsec/zq/proc/compiler"
	"github.com/brimsec/zq/proc/groupby"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zng/resolver"
	"go.uber.org/zap"
)

// WorkerURLs, if not empty, causes this process to
// implement parallelism using worker processes
// instead of goroutines.
var WorkerURLs []string

// XXX ReaderSortKey should be a field.Static.  Issue #1467.
type Config struct {
	Custom            compiler.Hook
	Logger            *zap.Logger
	ReaderSortKey     string
	ReaderSortReverse bool
	Span              nano.Span
	StatsTick         <-chan time.Time
	Warnings          chan string
}

func zbufDirInt(reversed bool) int {
	if reversed {
		return -1
	}
	return 1
}

var passProc = &ast.PassProc{Node: ast.Node{"PassProc"}}

func programPrep(program ast.Proc, sortKey field.Static, sortReversed bool) (ast.BooleanExpr, ast.Proc) {
	if program == nil {
		return nil, passProc
	}
	ReplaceGroupByProcDurationWithKey(program)
	if sortKey != nil {
		setGroupByProcInputSortDir(program, sortKey, zbufDirInt(sortReversed))
	}
	return liftFilter(program)
}

type scannerProc struct {
	zbuf.Scanner
}

func (s *scannerProc) Done() {}

type namedScanner struct {
	zbuf.Scanner
	name string
}

func (n *namedScanner) Pull() (zbuf.Batch, error) {
	b, err := n.Scanner.Pull()
	if err != nil {
		err = fmt.Errorf("%s: %w", n.name, err)
	}
	return b, err
}

func compile(ctx context.Context, program ast.Proc, zctx *resolver.Context, readers []zbuf.Reader, cfg Config) (*muxOutput, error) {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.Span.Dur == 0 {
		cfg.Span = nano.MaxSpan
	}
	if cfg.Warnings == nil {
		cfg.Warnings = make(chan string, 5)
	}

	filterExpr, program := programPrep(program, field.Dotted(cfg.ReaderSortKey), cfg.ReaderSortReverse)
	procs := make([]proc.Interface, 0, len(readers))
	scanners := make([]zbuf.Scanner, 0, len(readers))
	for _, r := range readers {
		sn, err := zbuf.NewScanner(ctx, r, filterExpr, cfg.Span)
		if err != nil {
			return nil, err
		}
		if stringer, ok := r.(fmt.Stringer); ok {
			sn = &namedScanner{sn, stringer.String()}
		}
		scanners = append(scanners, sn)
		procs = append(procs, &scannerProc{sn})
	}

	pctx := &proc.Context{
		Context:     ctx,
		TypeContext: zctx,
		Logger:      cfg.Logger,
		Warnings:    cfg.Warnings,
	}
	leaves, err := compiler.Compile(cfg.Custom, program, pctx, procs)
	if err != nil {
		return nil, err
	}
	return newMuxOutput(pctx, leaves, zbuf.MultiStats(scanners)), nil
}

type MultiConfig struct {
	Custom      compiler.Hook
	Order       zbuf.Order
	Logger      *zap.Logger
	Parallelism int
	Span        nano.Span
	StatsTick   <-chan time.Time
	Warnings    chan string
}

func compileMulti(ctx context.Context, program ast.Proc, zctx *resolver.Context, msrc MultiSource, mcfg MultiConfig) (*muxOutput, error) {
	if mcfg.Logger == nil {
		mcfg.Logger = zap.NewNop()
	}
	if mcfg.Span.Dur == 0 {
		mcfg.Span = nano.MaxSpan
	}
	if mcfg.Warnings == nil {
		mcfg.Warnings = make(chan string, 5)
	}

	if mcfg.Parallelism == 0 {
		// If mcfg.Parallelism has not been set by external configuration,
		// then it will be zero here.
		if len(WorkerURLs) > 0 {
			// If zqd has been started as a "root" process,
			// there is a -worker parameter with a list of WorkerURLs.
			// In this case, initialize Parallelism as the number of workers.
			mcfg.Parallelism = len(WorkerURLs)
		} else {
			// Otherwise, we will use threads (goroutines) for parallelism,
			// so initialize Parallelism based on
			// runtime configuation of max threads.
			mcfg.Parallelism = runtime.GOMAXPROCS(0)
		}
	}

	sortKey, sortReversed := msrc.OrderInfo()
	filterExpr, program := programPrep(program, sortKey, sortReversed)

	var isParallel bool
	if mcfg.Parallelism > 1 {
		program, isParallel = parallelizeFlowgraph(ensureSequentialProc(program), mcfg.Parallelism, sortKey, sortReversed)
	}
	if !isParallel {
		mcfg.Parallelism = 1
	}

	pctx := &proc.Context{
		Context:     ctx,
		TypeContext: zctx,
		Logger:      mcfg.Logger,
		Warnings:    mcfg.Warnings,
	}
	sources, pgroup, err := createParallelGroup(pctx, filterExpr, msrc, mcfg, WorkerURLs)
	if err != nil {
		return nil, err
	}
	leaves, err := compiler.Compile(mcfg.Custom, program, pctx, sources)
	if err != nil {
		return nil, err
	}
	return newMuxOutput(pctx, leaves, pgroup), nil
}

func ensureSequentialProc(p ast.Proc) *ast.SequentialProc {
	if p, ok := p.(*ast.SequentialProc); ok {
		return p
	}
	return &ast.SequentialProc{
		Procs: []ast.Proc{p},
	}
}

// liftFilter removes the filter at the head of the flowgraph AST, if
// one is present, and returns its ast.BooleanExpr and the modified
// flowgraph AST. If the flowgraph does not start with a filter, it
// returns nil and the unmodified flowgraph.
func liftFilter(p ast.Proc) (ast.BooleanExpr, ast.Proc) {
	if fp, ok := p.(*ast.FilterProc); ok {
		return fp.Filter, passProc
	}
	seq, ok := p.(*ast.SequentialProc)
	if ok && len(seq.Procs) > 0 {
		if fp, ok := seq.Procs[0].(*ast.FilterProc); ok {
			rest := ast.Proc(passProc)
			if len(seq.Procs) > 1 {
				rest = &ast.SequentialProc{
					Node:  ast.Node{"SequentialProc"},
					Procs: seq.Procs[1:],
				}
			}
			return fp.Filter, rest
		}
	}
	return nil, p
}

func filterToProc(be ast.BooleanExpr) ast.Proc {
	return &ast.FilterProc{
		Node:   ast.Node{Op: "FilterProc"},
		Filter: be,
	}
}

func ReplaceGroupByProcDurationWithKey(p ast.Proc) {
	switch p := p.(type) {
	case *ast.GroupByProc:
		if duration := p.Duration.Seconds; duration != 0 {
			durationKey := ast.Assignment{
				LHS: ast.NewDotExpr(field.New("ts")),
				RHS: &ast.FunctionCall{
					Node:     ast.Node{"FunctionCall"},
					Function: "trunc",
					Args: []ast.Expression{
						ast.NewDotExpr(field.New("ts")),
						&ast.Literal{
							Node:  ast.Node{"Literal"},
							Type:  "int64",
							Value: strconv.Itoa(duration),
						}},
				},
			}
			p.Keys = append([]ast.Assignment{durationKey}, p.Keys...)
		}
	case *ast.ParallelProc:
		for _, pp := range p.Procs {
			ReplaceGroupByProcDurationWithKey(pp)
		}
	case *ast.SequentialProc:
		for _, pp := range p.Procs {
			ReplaceGroupByProcDurationWithKey(pp)
		}
	}
}

func eq(e ast.Expression, b field.Static) bool {
	a, ok := ast.DotExprToField(e)
	if !ok {
		return false
	}
	return a.Equal(b)
}

// setGroupByProcInputSortDir examines p under the assumption that its input is
// sorted according to inputSortField and inputSortDir.  If p is an
// ast.GroupByProc and setGroupByProcInputSortDir can determine that its first
// grouping key is inputSortField or an order-preserving function of
// inputSortField, setGroupByProcInputSortDir sets ast.GroupByProc.InputSortDir
// to inputSortDir.  setGroupByProcInputSortDir returns true if it determines
// that p's output will remain sorted according to inputSortField and
// inputSortDir; otherwise, it returns false.
func setGroupByProcInputSortDir(p ast.Proc, inputSortField field.Static, inputSortDir int) bool {
	switch p := p.(type) {
	case *ast.CutProc:
		// Return true if the output record contains inputSortField.
		for _, f := range p.Fields {
			if eq(f.RHS, inputSortField) {
				return !p.Complement
			}
		}
		return p.Complement
	case *ast.GroupByProc:
		// Set p.InputSortDir and return true if the first grouping key
		// is inputSortField or an order-preserving function of it.
		if len(p.Keys) > 0 && eq(p.Keys[0].LHS, inputSortField) {
			rhs, ok := ast.DotExprToField(p.Keys[0].RHS)
			if ok && rhs.Equal(inputSortField) {
				p.InputSortDir = inputSortDir
				return true
			}
			if expr, ok := p.Keys[0].RHS.(*ast.FunctionCall); ok {
				switch expr.Function {
				case "ceil", "floor", "round", "trunc":
					if len(expr.Args) == 0 {
						return false
					}
					arg0, ok := ast.DotExprToField(expr.Args[0])
					if ok && arg0.Equal(inputSortField) {
						p.InputSortDir = inputSortDir
						return true
					}
				}
			}
		}
		return false
	case *ast.PutProc:
		for _, c := range p.Clauses {
			lhs, ok := ast.DotExprToField(c.LHS)
			if ok && lhs.Equal(inputSortField) {
				// XXX what if put field is not static and
				// computes to a collision...
				// Henri please check and I will remove on PR
				return false
			}
		}
		return true
	case *ast.SequentialProc:
		for _, pp := range p.Procs {
			if !setGroupByProcInputSortDir(pp, inputSortField, inputSortDir) {
				return false
			}
		}
		return true
	case *ast.FilterProc, *ast.HeadProc, *ast.PassProc, *ast.UniqProc, *ast.TailProc, *ast.FuseProc:
		return true
	default:
		return false
	}
}

// expressionFields returns a slice with all fields referenced
// in an expression. Fields will be repeated if they appear
// repeatedly.
func expressionFields(e ast.Expression) []ast.Expression {
	switch e := e.(type) {
	case *ast.UnaryExpression:
		return expressionFields(e.Operand)
	case *ast.BinaryExpression:
		if e.Operator == "." {
			// Just capture the whole dot expression.
			// When the colset computation happens, we will figure
			// out if this isn't a statically defined field expr.
			return []ast.Expression{e}
		}
		return append(expressionFields(e.LHS), expressionFields(e.RHS)...)
	case *ast.ConditionalExpression:
		fields := expressionFields(e.Condition)
		fields = append(fields, expressionFields(e.Then)...)
		fields = append(fields, expressionFields(e.Else)...)
		return fields
	case *ast.FunctionCall:
		var exprs []ast.Expression
		for _, arg := range e.Args {
			exprs = append(exprs, expressionFields(arg)...)
		}
		return exprs
	case *ast.CastExpression:
		return expressionFields(e.Expr)
	case *ast.Literal:
		return nil
	case *ast.Identifier:
		// This shouldn't happen as should raise an error but this
		// function is not wired to do so.  Maybe we just don't try
		// to optimize an semantically invalid AST?
		return nil
	default:
		panic("expression type not handled")
	}
}

// booleanExpressionFields returns a slice with all fields referenced
// in a boolean expression. Fields will be repeated if they appear
// repeatedly.  If all fields are referenced, nil is returned.
func booleanExpressionFields(e ast.BooleanExpr) []ast.Expression {
	switch e := e.(type) {
	case *ast.Search:
		return nil
	case *ast.LogicalAnd:
		l := booleanExpressionFields(e.Left)
		r := booleanExpressionFields(e.Right)
		if l == nil || r == nil {
			return nil
		}
		return append(l, r...)
	case *ast.LogicalOr:
		l := booleanExpressionFields(e.Left)
		r := booleanExpressionFields(e.Right)
		if l == nil || r == nil {
			return nil
		}
		return append(l, r...)
	case *ast.LogicalNot:
		return booleanExpressionFields(e.Expr)
	case *ast.MatchAll:
		// empty slice means match all, but nil means don't know
		return []ast.Expression{}
	case *ast.CompareAny:
		return nil
	case *ast.CompareField:
		return expressionFields(e.Field)
	default:
		panic("boolean expression type not handled")
	}
}

// computeColumns walks a flowgraph and computes a subset of columns
// that can be read by the source without modifying the output. For
// example, for the flowgraph "* | cut x", only the column "x" needs
// to be read by the source. On the other hand, for the flowgraph "* >
// 1", all columns need to be read.
//
// The return value is a map where the keys are string representations
// of the columns to be read at the source. If the return value is a
// nil map, all columns must be read.
func computeColumns(p ast.Proc) *Colset {
	cols, _ := computeColumnsR(p, newColset())
	return cols
}

// computeColumnsR is the recursive func used by computeColumns to
// compute a column set that can be read at the source. It walks a
// flowgraph, from the source, until it hits a "boundary proc". A
// "boundary proc" is one for which we can identify a set of input columns
// that fully determine its output. For example, 'cut x' is boundary
// proc (with set {x}); 'filter *>1' is a boundary proc (with set "all
// fields"); and 'head' is not a boundary proc.
// The first return value is a map representing the column set; the
// second is bool indicating that a boundary proc has been reached.
//
// Note that this function does not calculate the smallest column set
// for all possible flowgraphs: (1) It does not walk into parallel
// procs. (2) It does not track field renames: 'rename foo=y | count()
// by x' gets the column set {x, y} which is greater than the minimal
// column set {x}. (However 'rename x=y | count() by x' also gets {x,
// y}, which is minimal).
func computeColumnsR(p ast.Proc, colset *Colset) (*Colset, bool) {
	switch p := p.(type) {
	case *ast.CutProc:
		if p.Complement {
			return colset, false
		}
		for _, f := range p.Fields {
			if ok := colset.Add(&f); !ok {
				return colset, false
			}
		}
		return colset, true
	case *ast.GroupByProc:
		for _, r := range p.Reducers {
			reducer, ok := r.RHS.(*ast.Reducer)
			if !ok {
				// Illegal AST.
				return nil, false
			}
			if reducer.Expr != nil {
				if ok := colset.Add(reducer.Expr); !ok {
					return colset, false
				}
			}
			if reducer.Where != nil {
				if ok := colset.Add(reducer.Where); !ok {
					return colset, false
				}
			}
		}
		for _, key := range p.Keys {
			for _, field := range expressionFields(key.RHS) {
				if ok := colset.Add(field); !ok {
					return colset, false
				}
			}
		}
		return colset, true
	case *ast.SequentialProc:
		for _, pp := range p.Procs {
			var done bool
			colset, done = computeColumnsR(pp, colset)
			if done {
				return colset, true
			}
		}
		// We got to end without seeing a boundary proc, return "all cols"
		return nil, true
	case *ast.JoinProc, *ast.ParallelProc:
		// (These could be further analysed to determine the
		// colsets on each branch, and then merge them at the
		// split point.)
		return nil, true
	case *ast.UniqProc, *ast.FuseProc:
		return nil, true
	case *ast.HeadProc, *ast.TailProc, *ast.PassProc:
		return colset, false
	case *ast.FilterProc:
		fields := booleanExpressionFields(p.Filter)
		if fields == nil {
			return nil, true
		}
		for _, field := range fields {
			if ok := colset.Add(field); !ok {
				//XXX?
				// Henri please check and I will remove on PR
				return nil, false
			}
		}
		return colset, false
	case *ast.PutProc:
		for _, c := range p.Clauses {
			for _, field := range expressionFields(c.RHS) {
				if ok := colset.Add(field); !ok {
					//XXX?
					// Henri please check and I will remove on PR
					return nil, false
				}
			}
		}
		return colset, false
	case *ast.RenameProc:
		for _, f := range p.Fields {
			if ok := colset.Add(f.RHS); !ok {
				//XXX?
				// Henri please check and I will remove on PR
				return nil, false
			}
		}
		return colset, false
	case *ast.SortProc:
		if len(p.Fields) == 0 {
			// we don't know which sort field will
			// be used.
			return nil, true
		}
		for _, f := range p.Fields {
			if ok := colset.Add(f); !ok {
				//XXX?
				// Henri please check and I will remove on PR
				return nil, false
			}
		}
		return colset, false
	default:
		panic("proc type not handled")
	}
}

func copyProcs(ps []ast.Proc) []ast.Proc {
	var copies []ast.Proc
	for _, p := range ps {
		b, err := json.Marshal(p)
		if err != nil {
			panic(err)
		}
		proc, err := ast.UnpackJSON(nil, b)
		if err != nil {
			panic(err)
		}
		copies = append(copies, proc)
	}
	return copies
}

func buildSplitFlowgraph(branch, tail []ast.Proc, mergeField field.Static, reverse bool, N int) *ast.SequentialProc {
	if len(tail) == 0 && mergeField != nil {
		// Insert a pass tail in order to force a merge of the
		// parallel branches when compiling. (Trailing parallel branches are wired to
		// a mux output).
		tail = []ast.Proc{&ast.PassProc{Node: ast.Node{"PassProc"}}}
	}
	pp := &ast.ParallelProc{
		Node:              ast.Node{"ParallelProc"},
		Procs:             []ast.Proc{},
		MergeOrderField:   mergeField,
		MergeOrderReverse: reverse,
	}
	for i := 0; i < N; i++ {
		pp.Procs = append(pp.Procs, &ast.SequentialProc{
			Node:  ast.Node{"SequentialProc"},
			Procs: copyProcs(branch),
		})
	}
	return &ast.SequentialProc{
		Node:  ast.Node{"SequentialProc"},
		Procs: append([]ast.Proc{pp}, tail...),
	}
}

// parallelizeFlowgraph takes a sequential proc AST and tries to
// parallelize it by splitting as much as possible of the sequence
// into N parallel branches. The boolean return argument indicates
// whether the flowgraph could be parallelized.
func parallelizeFlowgraph(seq *ast.SequentialProc, N int, inputSortField field.Static, inputSortReversed bool) (*ast.SequentialProc, bool) {
	orderSensitiveTail := true
	for i := range seq.Procs {
		switch seq.Procs[i].(type) {
		case *ast.SortProc, *ast.GroupByProc:
			orderSensitiveTail = false
			break
		default:
			continue
		}
	}
	for i := range seq.Procs {
		switch p := seq.Procs[i].(type) {
		case *ast.FilterProc, *ast.PassProc:
			// Stateless procs: continue until we reach one of the procs below at
			// which point we'll either split the flowgraph or see we can't and return it as-is.
			continue
		case *ast.CutProc:
			if inputSortField == nil || !orderSensitiveTail {
				continue
			}
			if p.Complement {
				for _, f := range p.Fields {
					if eq(f.RHS, inputSortField) {
						return buildSplitFlowgraph(seq.Procs[0:i], seq.Procs[i:], inputSortField, inputSortReversed, N), true
					}
				}
				continue
			}
			var found bool
			for _, f := range p.Fields {
				fieldName, okField := ast.DotExprToField(f.RHS)
				lhs, okLHS := ast.DotExprToField(f.LHS)
				if okField && !fieldName.Equal(inputSortField) && okLHS && lhs.Equal(inputSortField) {
					return buildSplitFlowgraph(seq.Procs[0:i], seq.Procs[i:], inputSortField, inputSortReversed, N), true
				}
				if okField && fieldName.Equal(inputSortField) && lhs == nil {
					found = true
				}
			}
			if !found {
				return buildSplitFlowgraph(seq.Procs[0:i], seq.Procs[i:], inputSortField, inputSortReversed, N), true
			}
		case *ast.PutProc:
			if inputSortField == nil || !orderSensitiveTail {
				continue
			}
			for _, c := range p.Clauses {
				if eq(c.LHS, inputSortField) {
					return buildSplitFlowgraph(seq.Procs[0:i], seq.Procs[i:], inputSortField, inputSortReversed, N), true
				}
			}
			continue
		case *ast.RenameProc:
			if inputSortField == nil || !orderSensitiveTail {
				continue
			}
			for _, f := range p.Fields {
				if eq(f.LHS, inputSortField) {
					return buildSplitFlowgraph(seq.Procs[0:i], seq.Procs[i:], inputSortField, inputSortReversed, N), true
				}
			}
		case *ast.GroupByProc:
			if !groupby.IsDecomposable(p.Reducers) {
				return buildSplitFlowgraph(seq.Procs[0:i], seq.Procs[i:], inputSortField, inputSortReversed, N), true
			}
			// We have a decomposable groupby and can split the flowgraph into branches that run up to and including a groupby,
			// followed by a post-merge groupby that composes the results.
			var mergeField field.Static
			if p.Duration.Seconds != 0 {
				// Group by time requires a time-ordered merge, irrespective of any upstream ordering.
				mergeField = field.New("ts")
			}
			branch := copyProcs(seq.Procs[0 : i+1])
			branch[len(branch)-1].(*ast.GroupByProc).EmitPart = true

			composerGroupBy := copyProcs([]ast.Proc{p})[0].(*ast.GroupByProc)
			composerGroupBy.ConsumePart = true

			return buildSplitFlowgraph(branch, append([]ast.Proc{composerGroupBy}, seq.Procs[i+1:]...), mergeField, false, N), true
		case *ast.SortProc:
			dir := map[int]bool{-1: true, 1: false}[p.SortDir]
			if len(p.Fields) == 1 {
				// Single sort field: we can sort in each parallel branch, and then do an ordered merge.
				mergeField, ok := ast.DotExprToField(p.Fields[0])
				if !ok {
					// XXX is this right?
					return seq, false
				}
				return buildSplitFlowgraph(seq.Procs[0:i+1], seq.Procs[i+1:], mergeField, dir, N), true
			} else {
				// Unknown or multiple sort fields: we sort after the merge point, which can be unordered.
				return buildSplitFlowgraph(seq.Procs[0:i], seq.Procs[i:], nil, dir, N), true
			}
		case *ast.ParallelProc:
			return seq, false
		case *ast.HeadProc, *ast.TailProc:
			if inputSortField == nil {
				// Unknown order: we can't parallelize because we can't maintain this unknown order at the merge point.
				return seq, false
			}
			// put one head/tail on each parallel branch and one after the merge.
			return buildSplitFlowgraph(seq.Procs[0:i+1], seq.Procs[i:], inputSortField, inputSortReversed, N), true
		case *ast.UniqProc, *ast.FuseProc:
			if inputSortField == nil {
				// Unknown order: we can't parallelize because we can't maintain this unknown order at the merge point.
				return seq, false
			}
			// Split all the upstream procs into parallel branches, then merge and continue with this and any remaining procs.
			return buildSplitFlowgraph(seq.Procs[0:i], seq.Procs[i:], inputSortField, inputSortReversed, N), true
		case *ast.SequentialProc:
			return seq, false
			// XXX Joins can be parallelized but we need to write
			// the code to parallelize the flow graph, which is a bit
			// different from how group-bys are parallelized.
		case *ast.JoinProc:
			return seq, false
		default:
			panic("proc type not handled")
		}
	}
	// If we're here, we reached the end of the flowgraph without
	// coming across a merge-forcing proc. If inputs are sorted,
	// we can parallelize the entire chain and do an ordered
	// merge. Otherwise, no parallelization.
	if inputSortField == nil {
		return seq, false
	}
	return buildSplitFlowgraph(seq.Procs, nil, inputSortField, inputSortReversed, N), true
}
