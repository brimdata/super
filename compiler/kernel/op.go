package kernel

import (
	"context"
	"errors"
	"fmt"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/compiler/ast/dag"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/pkg/field"
	"github.com/brimdata/zed/runtime/expr"
	"github.com/brimdata/zed/runtime/expr/extent"
	"github.com/brimdata/zed/runtime/op"
	"github.com/brimdata/zed/runtime/op/combine"
	"github.com/brimdata/zed/runtime/op/explode"
	"github.com/brimdata/zed/runtime/op/exprswitch"
	"github.com/brimdata/zed/runtime/op/fork"
	"github.com/brimdata/zed/runtime/op/from"
	"github.com/brimdata/zed/runtime/op/fuse"
	"github.com/brimdata/zed/runtime/op/head"
	"github.com/brimdata/zed/runtime/op/join"
	"github.com/brimdata/zed/runtime/op/merge"
	"github.com/brimdata/zed/runtime/op/pass"
	"github.com/brimdata/zed/runtime/op/shape"
	"github.com/brimdata/zed/runtime/op/sort"
	"github.com/brimdata/zed/runtime/op/switcher"
	"github.com/brimdata/zed/runtime/op/tail"
	"github.com/brimdata/zed/runtime/op/top"
	"github.com/brimdata/zed/runtime/op/traverse"
	"github.com/brimdata/zed/runtime/op/uniq"
	"github.com/brimdata/zed/runtime/op/yield"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zson"
)

var ErrJoinParents = errors.New("join requires two upstream parallel query paths")

type Builder struct {
	pctx       *op.Context
	adaptor    op.DataAdaptor
	schedulers map[dag.Source]op.Scheduler
}

func NewBuilder(pctx *op.Context, adaptor op.DataAdaptor) *Builder {
	return &Builder{
		pctx:       pctx,
		adaptor:    adaptor,
		schedulers: make(map[dag.Source]op.Scheduler),
	}
}

type Reader struct {
	Layout  order.Layout
	Readers []zio.Reader
}

var _ dag.Source = (*Reader)(nil)

func (*Reader) Source() {}

func (b *Builder) Build(seq *dag.Sequential) ([]zbuf.Puller, error) {
	if !seq.IsEntry() {
		return nil, errors.New("internal error: DAG entry point is not a data source")
	}
	return b.compile(seq, nil)
}

func (b *Builder) zctx() *zed.Context {
	return b.pctx.Zctx
}

func (b *Builder) Meters() []zbuf.Meter {
	var meters []zbuf.Meter
	for _, sched := range b.schedulers {
		meters = append(meters, sched)
	}
	return meters
}

func (b *Builder) compileLeaf(o dag.Op, parent zbuf.Puller) (zbuf.Puller, error) {
	switch v := o.(type) {
	case *dag.Summarize:
		return b.compileGroupBy(parent, v)
	case *dag.Cut:
		assignments, err := b.compileAssignments(v.Args)
		if err != nil {
			return nil, err
		}
		lhs, rhs := splitAssignments(assignments)
		cutter, err := expr.NewCutter(b.pctx.Zctx, lhs, rhs)
		if err != nil {
			return nil, err
		}
		if v.Quiet {
			cutter.Quiet()
		}
		return op.NewApplier(b.pctx, parent, cutter), nil
	case *dag.Drop:
		if len(v.Args) == 0 {
			return nil, errors.New("drop: no fields given")
		}
		fields := make(field.List, 0, len(v.Args))
		for _, e := range v.Args {
			field, ok := e.(*dag.This)
			if !ok {
				return nil, errors.New("drop: arg not a field")
			}
			fields = append(fields, field.Path)
		}
		dropper := expr.NewDropper(b.pctx.Zctx, fields)
		return op.NewApplier(b.pctx, parent, dropper), nil
	case *dag.Sort:
		fields, err := b.compileExprs(v.Args)
		if err != nil {
			return nil, err
		}
		sort, err := sort.New(b.pctx, parent, fields, v.Order, v.NullsFirst)
		if err != nil {
			return nil, fmt.Errorf("compiling sort: %w", err)
		}
		return sort, nil
	case *dag.Head:
		limit := v.Count
		if limit == 0 {
			limit = 1
		}
		return head.New(parent, limit), nil
	case *dag.Tail:
		limit := v.Count
		if limit == 0 {
			limit = 1
		}
		return tail.New(parent, limit), nil
	case *dag.Uniq:
		return uniq.New(b.pctx, parent, v.Cflag), nil
	case *dag.Pass:
		return pass.New(parent), nil
	case *dag.Filter:
		f, err := b.compileExpr(v.Expr)
		if err != nil {
			return nil, fmt.Errorf("compiling filter: %w", err)
		}
		return op.NewApplier(b.pctx, parent, expr.NewFilterApplier(b.pctx.Zctx, f)), nil
	case *dag.Top:
		fields, err := b.compileExprs(v.Args)
		if err != nil {
			return nil, fmt.Errorf("compiling top: %w", err)
		}
		return top.New(parent, b.pctx.Zctx, v.Limit, fields, v.Flush), nil
	case *dag.Put:
		clauses, err := b.compileAssignments(v.Args)
		if err != nil {
			return nil, err
		}
		putter, err := expr.NewPutter(b.pctx.Zctx, clauses)
		if err != nil {
			return nil, err
		}
		return op.NewApplier(b.pctx, parent, putter), nil
	case *dag.Rename:
		var srcs, dsts field.List
		for _, fa := range v.Args {
			dst, err := compileLval(fa.LHS)
			if err != nil {
				return nil, err
			}
			// We call CompileLval on the RHS because renames are
			// restricted to dotted field name expressions.
			src, err := compileLval(fa.RHS)
			if err != nil {
				return nil, err
			}
			if len(dst) != len(src) {
				return nil, fmt.Errorf("cannot rename %s to %s", src, dst)
			}
			// Check that the prefixes match and, if not, report first place
			// that they don't.
			for i := 0; i <= len(src)-2; i++ {
				if src[i] != dst[i] {
					return nil, fmt.Errorf("cannot rename %s to %s (differ in %s vs %s)", src, dst, src[i], dst[i])
				}
			}
			dsts = append(dsts, dst)
			srcs = append(srcs, src)
		}
		renamer := expr.NewRenamer(b.pctx.Zctx, srcs, dsts)
		return op.NewApplier(b.pctx, parent, renamer), nil
	case *dag.Fuse:
		return fuse.New(b.pctx, parent)
	case *dag.Shape:
		return shape.New(b.pctx, parent)
	case *dag.Join:
		return nil, ErrJoinParents
	case *dag.Merge:
		return nil, errors.New("merge: multiple upstream paths required")
	case *dag.Explode:
		typ, err := zson.ParseType(b.pctx.Zctx, v.Type)
		if err != nil {
			return nil, err
		}
		args, err := b.compileExprs(v.Args)
		if err != nil {
			return nil, err
		}
		as, err := compileLval(v.As)
		if len(as) != 1 {
			return nil, errors.New("explode field must be a top-level field")
		}
		return explode.New(b.pctx.Zctx, parent, args, typ, as.Leaf())
	case *dag.Over:
		return b.compileOver(parent, v, nil, nil)
	case *dag.Yield:
		exprs, err := b.compileExprs(v.Exprs)
		if err != nil {
			return nil, err
		}
		t := yield.New(parent, exprs)
		return t, nil
	case *dag.Let:
		if v.Over == nil {
			return nil, errors.New("let operator missing over expression in DAG")
		}
		names, exprs, err := b.compileLets(v.Defs)
		if err != nil {
			return nil, err
		}
		return b.compileOver(parent, v.Over, names, exprs)
	default:
		return nil, fmt.Errorf("unknown DAG operator type: %v", v)
	}
}

func (b *Builder) compileLets(defs []dag.Def) ([]string, []expr.Evaluator, error) {
	exprs := make([]expr.Evaluator, 0, len(defs))
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		e, err := b.compileExpr(def.Expr)
		if err != nil {
			return nil, nil, err
		}
		exprs = append(exprs, e)
		names = append(names, def.Name)
	}
	return names, exprs, nil
}

func (b *Builder) compileOver(parent zbuf.Puller, over *dag.Over, names []string, lets []expr.Evaluator) (zbuf.Puller, error) {
	exprs, err := b.compileExprs(over.Exprs)
	if err != nil {
		return nil, err
	}
	enter := traverse.NewOver(b.pctx, parent, exprs)
	if over.Scope == nil {
		return enter, nil
	}
	scope := enter.AddScope(b.pctx.Context, names, lets)
	exits, err := b.compile(over.Scope, []zbuf.Puller{scope})
	if err != nil {
		return nil, err
	}
	var exit zbuf.Puller
	if len(exits) == 1 {
		exit = exits[0]
	} else {
		// This can happen when output of over body
		// is a fork or switch.
		exit = combine.New(b.pctx, exits)
	}
	return scope.NewExit(exit), nil
}

func (b *Builder) compileAssignments(assignments []dag.Assignment) ([]expr.Assignment, error) {
	keys := make([]expr.Assignment, 0, len(assignments))
	for _, assignment := range assignments {
		a, err := b.compileAssignment(&assignment)
		if err != nil {
			return nil, err
		}
		keys = append(keys, a)
	}
	return keys, nil
}

func splitAssignments(assignments []expr.Assignment) (field.List, []expr.Evaluator) {
	n := len(assignments)
	lhs := make(field.List, 0, n)
	rhs := make([]expr.Evaluator, 0, n)
	for _, a := range assignments {
		lhs = append(lhs, a.LHS)
		rhs = append(rhs, a.RHS)
	}
	return lhs, rhs
}

func (b *Builder) compileSequential(seq *dag.Sequential, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	for _, o := range seq.Ops {
		var err error
		parents, err = b.compile(o, parents)
		if err != nil {
			return nil, err
		}
	}
	return parents, nil
}

func (b *Builder) compileParallel(parallel *dag.Parallel, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	if len(parents) == 0 {
		var ops []zbuf.Puller
		for _, o := range parallel.Ops {
			branch, err := b.compile(o, nil)
			if err != nil {
				return nil, err
			}
			ops = append(ops, branch...)
		}
		return ops, nil
	}
	n := len(parallel.Ops)
	if len(parents) == 1 {
		// Single parent: insert a fork for n-way fanout.
		parents = fork.New(b.pctx, parents[0], n)
	}
	if len(parents) != n {
		return nil, fmt.Errorf("parallel input mismatch: %d parents with %d flowgraph paths", len(parents), len(parallel.Ops))
	}
	var ops []zbuf.Puller
	for k := 0; k < n; k++ {
		op, err := b.compile(parallel.Ops[k], []zbuf.Puller{parents[k]})
		if err != nil {
			return nil, err
		}
		ops = append(ops, op...)
	}
	return ops, nil
}

func (b *Builder) compileExprSwitch(swtch *dag.Switch, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	if len(parents) != 1 {
		return nil, errors.New("expression switch has multiple parents")
	}
	e, err := b.compileExpr(swtch.Expr)
	if err != nil {
		return nil, err
	}
	s := exprswitch.New(b.pctx, parents[0], e)
	var exits []zbuf.Puller
	for _, c := range swtch.Cases {
		var val *zed.Value
		if c.Expr != nil {
			val, err = b.evalAtCompileTime(c.Expr)
			if err != nil {
				return nil, err
			}
			if val.IsError() {
				return nil, errors.New("switch case is not a constant expression")
			}
		}
		parents, err := b.compile(c.Op, []zbuf.Puller{s.AddCase(val)})
		if err != nil {
			return nil, err
		}
		exits = append(exits, parents...)
	}
	return exits, nil
}

func (b *Builder) compileSwitch(swtch *dag.Switch, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	n := len(swtch.Cases)
	if len(parents) == 1 {
		// Single parent: insert a switcher and wire to each branch.
		switcher := switcher.New(b.pctx, parents[0])
		parents = []zbuf.Puller{}
		for _, c := range swtch.Cases {
			f, err := b.compileExpr(c.Expr)
			if err != nil {
				return nil, fmt.Errorf("compiling switch case filter: %w", err)
			}
			sc := switcher.AddCase(f)
			parents = append(parents, sc)
		}
	}
	if len(parents) != n {
		return nil, fmt.Errorf("%d parents for switch with %d branches", len(parents), len(swtch.Cases))
	}
	var ops []zbuf.Puller
	for k := 0; k < n; k++ {
		o, err := b.compile(swtch.Cases[k].Op, []zbuf.Puller{parents[k]})
		if err != nil {
			return nil, err
		}
		ops = append(ops, o...)
	}
	return ops, nil
}

// compile compiles a DAG into a graph of runtime operators, and returns
// the leaves.
func (b *Builder) compile(o dag.Op, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	switch o := o.(type) {
	case *dag.Sequential:
		if len(o.Ops) == 0 {
			return nil, errors.New("empty sequential operator")
		}
		return b.compileSequential(o, parents)
	case *dag.Parallel:
		return b.compileParallel(o, parents)
	case *dag.Switch:
		if o.Expr != nil {
			return b.compileExprSwitch(o, parents)
		}
		return b.compileSwitch(o, parents)
	case *dag.From:
		if len(parents) > 1 {
			return nil, errors.New("'from' operator can have at most one parent")
		}
		var parent zbuf.Puller
		if len(parents) == 1 {
			parent = parents[0]
		}
		return b.compileFrom(o, parent)
	case *dag.Join:
		if len(parents) != 2 {
			return nil, ErrJoinParents
		}
		assignments, err := b.compileAssignments(o.Args)
		if err != nil {
			return nil, err
		}
		lhs, rhs := splitAssignments(assignments)
		leftKey, err := b.compileExpr(o.LeftKey)
		if err != nil {
			return nil, err
		}
		rightKey, err := b.compileExpr(o.RightKey)
		if err != nil {
			return nil, err
		}
		leftParent, rightParent := parents[0], parents[1]
		var anti, inner bool
		switch o.Style {
		case "anti":
			anti = true
		case "inner":
			inner = true
		case "left":
		case "right":
			leftKey, rightKey = rightKey, leftKey
			leftParent, rightParent = rightParent, leftParent
		default:
			return nil, fmt.Errorf("unknown kind of join: '%s'", o.Style)
		}
		join, err := join.New(b.pctx, anti, inner, leftParent, rightParent, leftKey, rightKey, lhs, rhs)
		if err != nil {
			return nil, err
		}
		return []zbuf.Puller{join}, nil
	case *dag.Merge:
		e, err := b.compileExpr(o.Expr)
		if err != nil {
			return nil, err
		}
		nullsMax := o.Order == order.Asc
		cmp := expr.NewComparator(nullsMax, !nullsMax, e).WithMissingAsNull()
		return []zbuf.Puller{merge.New(b.pctx, parents, cmp.Compare)}, nil
	default:
		var parent zbuf.Puller
		if len(parents) == 1 {
			parent = parents[0]
		} else {
			parent = combine.New(b.pctx, parents)
		}
		p, err := b.compileLeaf(o, parent)
		if err != nil {
			return nil, err
		}
		return []zbuf.Puller{p}, nil
	}
}

func (b *Builder) compileFrom(from *dag.From, parent zbuf.Puller) ([]zbuf.Puller, error) {
	var parents []zbuf.Puller
	var npass int
	for k := range from.Trunks {
		outputs, err := b.compileTrunk(&from.Trunks[k], parent)
		if err != nil {
			return nil, err
		}
		if _, ok := from.Trunks[k].Source.(*dag.Pass); ok {
			npass++
		}
		parents = append(parents, outputs...)
	}
	if parent == nil && npass > 0 {
		return nil, errors.New("no data source for 'from operator' pass-through branch")
	}
	if parent != nil {
		if npass > 1 {
			return nil, errors.New("cannot have multiple pass-through branches in 'from operator'")
		}
		if npass == 0 {
			return nil, errors.New("upstream data source blocked by 'from operator'")
		}
	}
	return parents, nil
}

func (b *Builder) compileTrunk(trunk *dag.Trunk, parent zbuf.Puller) ([]zbuf.Puller, error) {
	pushdown, err := b.PushdownOf(trunk)
	if err != nil {
		return nil, err
	}
	var source zbuf.Puller
	switch src := trunk.Source.(type) {
	case *Reader:
		sched := &readerScheduler{
			ctx:     b.pctx.Context,
			filter:  pushdown,
			readers: src.Readers,
		}
		source = from.NewScheduler(b.pctx, sched)
		b.schedulers[src] = sched
	case *dag.Pass:
		source = parent
	case *dag.Pool:
		// We keep a map of schedulers indexed by *dag.Pool so we
		// properly share parallel instances of a given scheduler
		// across different DAG entry points.  The scanners from a
		// common lake.ScanScheduler are distributed across the collection
		// of op.From operators.
		sched, ok := b.schedulers[src]
		if !ok {
			span, err := b.compileRange(src, src.ScanLower, src.ScanUpper)
			if err != nil {
				return nil, err
			}
			sched, err = b.adaptor.NewScheduler(b.pctx.Context, b.pctx.Zctx, src, span, pushdown)
			if err != nil {
				return nil, err
			}
			b.schedulers[src] = sched
		}
		source = from.NewScheduler(b.pctx, sched)
	case *dag.PoolMeta:
		sched, ok := b.schedulers[src]
		if !ok {
			sched, err = b.adaptor.NewScheduler(b.pctx.Context, b.pctx.Zctx, src, nil, pushdown)
			if err != nil {
				return nil, err
			}
			b.schedulers[src] = sched
		}
		source = from.NewScheduler(b.pctx, sched)
	case *dag.CommitMeta:
		sched, ok := b.schedulers[src]
		if !ok {
			span, err := b.compileRange(src, src.ScanLower, src.ScanUpper)
			if err != nil {
				return nil, err
			}
			sched, err = b.adaptor.NewScheduler(b.pctx.Context, b.pctx.Zctx, src, span, pushdown)
			if err != nil {
				return nil, err
			}
			b.schedulers[src] = sched
		}
		source = from.NewScheduler(b.pctx, sched)
	case *dag.LakeMeta:
		sched, ok := b.schedulers[src]
		if !ok {
			sched, err = b.adaptor.NewScheduler(b.pctx.Context, b.pctx.Zctx, src, nil, pushdown)
			if err != nil {
				return nil, err
			}
			b.schedulers[src] = sched
		}
		source = from.NewScheduler(b.pctx, sched)
	case *dag.HTTP:
		puller, err := b.adaptor.Open(b.pctx.Context, b.pctx.Zctx, src.URL, src.Format, pushdown)
		if err != nil {
			return nil, err
		}
		source = from.NewPuller(b.pctx, puller)
	case *dag.File:
		scanner, err := b.adaptor.Open(b.pctx.Context, b.pctx.Zctx, src.Path, src.Format, pushdown)
		if err != nil {
			return nil, err
		}
		source = from.NewPuller(b.pctx, scanner)
	default:
		return nil, fmt.Errorf("Builder.compileTrunk: unknown type: %T", src)
	}
	if trunk.Seq == nil {
		return []zbuf.Puller{source}, nil
	}
	return b.compileSequential(trunk.Seq, []zbuf.Puller{source})
}

func (b *Builder) compileRange(src dag.Source, exprLower, exprUpper dag.Expr) (extent.Span, error) {
	lower := &zed.Value{zed.TypeNull, nil}
	upper := &zed.Value{zed.TypeNull, nil}
	if exprLower != nil {
		var err error
		lower, err = b.evalAtCompileTime(exprLower)
		if err != nil {
			return nil, err
		}
	}
	if exprUpper != nil {
		var err error
		upper, err = b.evalAtCompileTime(exprUpper)
		if err != nil {
			return nil, err
		}
	}
	var span extent.Span
	if lower.Bytes != nil || upper.Bytes != nil {
		layout := b.adaptor.Layout(b.pctx.Context, src)
		span = extent.NewGenericFromOrder(*lower, *upper, layout.Order)
	}
	return span, nil
}

func (b *Builder) PushdownOf(trunk *dag.Trunk) (*Filter, error) {
	if trunk.Pushdown == nil {
		return nil, nil
	}
	f, ok := trunk.Pushdown.(*dag.Filter)
	if !ok {
		return nil, errors.New("non-filter pushdown operator not yet supported")
	}
	return &Filter{f.Expr, b}, nil
}

func (b *Builder) evalAtCompileTime(in dag.Expr) (val *zed.Value, err error) {
	if in == nil {
		return zed.Null, nil
	}
	e, err := b.compileExpr(in)
	if err != nil {
		return nil, err
	}
	// Catch panic as the runtime will panic if there is a
	// reference to a var not in scope, a field access null this, etc.
	defer func() {
		if recover() != nil {
			val = b.zctx().Missing()
		}
	}()
	return e.Eval(expr.NewContext(), b.zctx().Missing()), nil
}

func EvalAtCompileTime(zctx *zed.Context, in dag.Expr) (val *zed.Value, err error) {
	// We pass in a nil adaptor, which causes a panic for anything adaptor
	// related, which is not currently allowed in an expression sub-query.
	b := NewBuilder(op.NewContext(context.Background(), zctx, nil), nil)
	return b.evalAtCompileTime(in)
}

type readerScheduler struct {
	ctx      context.Context
	filter   zbuf.Filter
	readers  []zio.Reader
	scanner  zbuf.Scanner
	progress zbuf.Progress
}

func (r *readerScheduler) PullScanTask() (zbuf.Puller, error) {
	if r.scanner != nil {
		r.progress.Add(r.scanner.Progress())
		r.scanner = nil
	}
	if len(r.readers) == 0 {
		return nil, nil
	}
	zr := r.readers[0]
	r.readers = r.readers[1:]
	s, err := zbuf.NewScanner(r.ctx, zr, r.filter)
	if err != nil {
		return nil, err
	}
	r.scanner = s
	if stringer, ok := zr.(fmt.Stringer); ok {
		s = zbuf.NamedScanner(s, stringer.String())
	}
	return &donePuller{s, r}, nil
}

func (r *readerScheduler) Progress() zbuf.Progress {
	// Add the cumulative progress to the current scanner's progress.
	progress := r.progress
	if r.scanner != nil {
		progress.Add(r.scanner.Progress())
	}
	return progress
}

type donePuller struct {
	zbuf.Puller
	sched *readerScheduler
}

func (d *donePuller) Pull(done bool) (zbuf.Batch, error) {
	if done {
		d.sched.readers = nil
	}
	return d.Puller.Pull(done)
}
