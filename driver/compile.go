package driver

import (
	"context"
	"strconv"

	"github.com/brimsec/zq/ast"
	"github.com/brimsec/zq/expr"
	"github.com/brimsec/zq/filter"
	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/proc"
	"github.com/brimsec/zq/scanner"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zng/resolver"
	"go.uber.org/zap"
)

type Config struct {
	Custom            proc.Compiler
	Logger            *zap.Logger
	ReaderSortKey     string
	ReaderSortReverse bool
	Span              nano.Span
	Warnings          chan string
}

// Compile takes an AST, an input reader, and configuration parameters,
// and compiles it into a runnable flowgraph, returning a
// proc.MuxOutput that which brings together all of the flowgraphs
// tails, and is ready to be Pull()'d from.
func Compile(ctx context.Context, program ast.Proc, zctx *resolver.Context, reader zbuf.Reader, cfg Config) (*MuxOutput, error) {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.Span.Dur == 0 {
		cfg.Span = nano.MaxSpan
	}
	if cfg.Warnings == nil {
		cfg.Warnings = make(chan string, 5)
	}

	ReplaceGroupByProcDurationWithKey(program)
	if cfg.ReaderSortKey != "" {
		dir := 1
		if cfg.ReaderSortReverse {
			dir = -1
		}
		setGroupByProcInputSortDir(program, cfg.ReaderSortKey, dir)
	}
	filterAst, program := liftFilter(program)
	scanner, err := newScanner(ctx, reader, filterAst, cfg.Span)
	if err != nil {
		return nil, err
	}
	pctx := &proc.Context{
		Context:     ctx,
		TypeContext: zctx,
		Logger:      cfg.Logger,
		Warnings:    cfg.Warnings,
	}
	leaves, err := proc.CompileProc(cfg.Custom, program, pctx, scanner)
	if err != nil {
		return nil, err
	}
	return NewMuxOutput(pctx, leaves, scanner), nil
}

// liftFilter removes the filter at the head of the flowgraph AST, if
// one is present, and returns it and the modified flowgraph AST. If
// the flowgraph does not start with a filter, it returns nil and the
// unmodified flowgraph.
func liftFilter(p ast.Proc) (*ast.FilterProc, ast.Proc) {
	if fp, ok := p.(*ast.FilterProc); ok {
		pass := &ast.PassProc{
			Node: ast.Node{"PassProc"},
		}
		return fp, pass
	}
	seq, ok := p.(*ast.SequentialProc)
	if ok && len(seq.Procs) > 0 {
		if fp, ok := seq.Procs[0].(*ast.FilterProc); ok {
			rest := &ast.SequentialProc{
				Node:  ast.Node{"SequentialProc"},
				Procs: seq.Procs[1:],
			}
			return fp, rest
		}
	}
	return nil, p
}

func ReplaceGroupByProcDurationWithKey(p ast.Proc) {
	switch p := p.(type) {
	case *ast.GroupByProc:
		if duration := p.Duration.Seconds; duration != 0 {
			durationKey := ast.ExpressionAssignment{
				Target: "ts",
				Expr: &ast.FunctionCall{
					Function: "Time.trunc",
					Args: []ast.Expression{
						&ast.FieldRead{Field: "ts"},
						&ast.Literal{
							Type:  "int64",
							Value: strconv.Itoa(duration),
						}},
				},
			}
			p.Duration.Seconds = 0
			p.Keys = append([]ast.ExpressionAssignment{durationKey}, p.Keys...)
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

// setGroupByProcInputSortDir examines p under the assumption that its input is
// sorted according to inputSortField and inputSortDir.  If p is an
// ast.GroupByProc and setGroupByProcInputSortDir can determine that its first
// grouping key is inputSortField or an order-preserving function of
// inputSortField, setGroupByProcInputSortDir sets ast.GroupByProc.InputSortDir
// to inputSortDir.  setGroupByProcInputSortDir returns true if it determines
// that p's output will remain sorted according to inputSortField and
// inputSortDir; otherwise, it returns false.
func setGroupByProcInputSortDir(p ast.Proc, inputSortField string, inputSortDir int) bool {
	switch p := p.(type) {
	case *ast.CutProc:
		// Return true if the output record contains inputSortField.
		for _, f := range p.Fields {
			if f.Source == inputSortField {
				return !p.Complement
			}
		}
		return p.Complement
	case *ast.GroupByProc:
		// Set p.InputSortDir and return true if the first grouping key
		// is inputSortField or an order-preserving function of it.
		if len(p.Keys) > 0 && p.Keys[0].Target == inputSortField {
			switch expr := p.Keys[0].Expr.(type) {
			case *ast.FieldRead:
				if expr.Field == inputSortField {
					p.InputSortDir = inputSortDir
					return true
				}
			case *ast.FunctionCall:
				switch expr.Function {
				case "Math.ceil", "Math.floor", "Math.round", "Time.trunc":
					if len(expr.Args) > 0 {
						arg0, ok := expr.Args[0].(*ast.FieldRead)
						if ok && arg0.Field == inputSortField {
							p.InputSortDir = inputSortDir
							return true
						}
					}
				}
			}
		}
		return false
	case *ast.PutProc:
		for _, c := range p.Clauses {
			if c.Target == inputSortField {
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
	case *ast.FilterProc, *ast.HeadProc, *ast.PassProc, *ast.UniqProc, *ast.TailProc:
		return true
	default:
		return false
	}
}

// expressionFields returns a slice with all fields referenced
// in an expression. Fields will be repeated if they appear
// repeatedly.
func expressionFields(expr ast.Expression) []string {
	switch expr := expr.(type) {
	case *ast.UnaryExpression:
		return expressionFields(expr.Operand)
	case *ast.BinaryExpression:
		lhs := expressionFields(expr.LHS)
		rhs := expressionFields(expr.RHS)
		return append(lhs, rhs...)
	case *ast.ConditionalExpression:
		fields := expressionFields(expr.Condition)
		fields = append(fields, expressionFields(expr.Then)...)
		fields = append(fields, expressionFields(expr.Else)...)
		return fields
	case *ast.FunctionCall:
		fields := []string{}
		for _, arg := range expr.Args {
			fields = append(fields, expressionFields(arg)...)
		}
		return fields
	case *ast.CastExpression:
		return expressionFields(expr.Expr)
	case *ast.Literal:
		return []string{}
	case *ast.FieldRead:
		return []string{expr.Field}
	case *ast.FieldCall:
		return expressionFields(expr.Field.(ast.Expression))
	default:
		panic("expression type not handled")
	}
}

// booleanExpressionFields returns a slice with all fields referenced
// in a boolean expression. Fields will be repeated if they appear
// repeatedly.  If all fields are referenced, nil is returned.
func booleanExpressionFields(expr ast.BooleanExpr) []string {
	switch expr := expr.(type) {
	case *ast.Search:
		return nil
	case *ast.LogicalAnd:
		l := booleanExpressionFields(expr.Left)
		r := booleanExpressionFields(expr.Right)
		if l == nil || r == nil {
			return nil
		}
		return append(l, r...)
	case *ast.LogicalOr:
		l := booleanExpressionFields(expr.Left)
		r := booleanExpressionFields(expr.Right)
		if l == nil || r == nil {
			return nil
		}
		return append(l, r...)
	case *ast.LogicalNot:
		return booleanExpressionFields(expr.Expr)
	case *ast.MatchAll:
		return []string{}
	case *ast.CompareAny:
		return nil
	case *ast.CompareField:
		return expressionFields(expr.Field.(ast.Expression))
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
func computeColumns(p ast.Proc) map[string]struct{} {
	cols, _ := computeColumnsR(p, map[string]struct{}{})
	return cols
}

// computeColumnsR is the recursive func used by computeColumns to
// compute a column set that can be read at the source. It walks a
// flowgraph, from the source, until it hits a "boundary proc". A
// "boundary proc" is one for which we can identify a set of input columns
// that fully determine its output. For example, 'cut x' is boundary
// proc (with set {x}); 'filter *>1' is a boundary proc (with set "all
// fields"); and 'head' is not a boundary proc.
//
// Note that this function does not calculate the smallest column set
// for all possible flowgraphs: (1) It does not walk into parallel
// procs. (2) It does not track field renames: 'rename foo=y | count()
// by x' gets the column set {x, y} which is greater than the minimal
// column set {x}. (However 'rename x=y | count() by x' also gets {x,
// y}, which is minimal).
func computeColumnsR(p ast.Proc, colset map[string]struct{}) (map[string]struct{}, bool) {
	switch p := p.(type) {
	case *ast.CutProc:
		if p.Complement {
			return colset, false
		}
		for _, f := range p.Fields {
			colset[f.Source] = struct{}{}
		}
		return colset, true
	case *ast.GroupByProc:
		for _, r := range p.Reducers {
			if r.Field == nil {
				continue
			}
			colset[expr.FieldExprToString(r.Field)] = struct{}{}
		}
		for _, key := range p.Keys {
			for _, field := range expressionFields(key.Expr) {
				colset[field] = struct{}{}
			}
		}
		return colset, true
	case *ast.ReduceProc:
		for _, r := range p.Reducers {
			if r.Field == nil {
				continue
			}
			colset[expr.FieldExprToString(r.Field)] = struct{}{}
		}
		return colset, true
	case *ast.SequentialProc:
		for i := range p.Procs {
			var done bool
			colset, done = computeColumnsR(p.Procs[i], colset)
			if done {
				return colset, true
			}
		}
		// We got to end without seeing a boundary proc, return "all cols"
		return nil, true
	case *ast.ParallelProc:
		// (These could be further analysed to determine the
		// colsets on each branch, and then merge them at the
		// split point.)
		return nil, true
	case *ast.UniqProc:
		return nil, true
	case *ast.HeadProc, *ast.TailProc, *ast.PassProc:
		return colset, false
	case *ast.FilterProc:
		fields := booleanExpressionFields(p.Filter)
		if fields == nil {
			return nil, true
		}
		for _, field := range fields {
			colset[field] = struct{}{}
		}
		return colset, false
	case *ast.PutProc:
		for _, c := range p.Clauses {
			for _, field := range expressionFields(c.Expr) {
				colset[field] = struct{}{}
			}
		}
		return colset, false
	case *ast.RenameProc:
		for _, f := range p.Fields {
			colset[f.Source] = struct{}{}
		}
		return colset, false
	case *ast.SortProc:
		if len(p.Fields) == 0 {
			// we don't know which sort field will
			// be used.
			return nil, true
		}
		for _, f := range p.Fields {
			colset[expr.FieldExprToString(f)] = struct{}{}
		}
		return colset, false
	default:
		panic("proc type not handled")
	}
}

// newScanner takes a Reader, optional Filter AST, and timespan, and
// constructs a scanner that can be used as the head of a
// flowgraph.
func newScanner(ctx context.Context, reader zbuf.Reader, fltast *ast.FilterProc, span nano.Span) (*scanner.Scanner, error) {
	var f filter.Filter
	if fltast != nil {
		var err error
		if f, err = filter.Compile(fltast.Filter); err != nil {
			return nil, err
		}
	}
	return scanner.NewScanner(ctx, reader, f, span), nil
}
