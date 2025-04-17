package kernel

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/optimizer"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/runtime/sam/op/combine"
	"github.com/brimdata/super/runtime/sam/op/distinct"
	"github.com/brimdata/super/runtime/sam/op/explode"
	"github.com/brimdata/super/runtime/sam/op/exprswitch"
	"github.com/brimdata/super/runtime/sam/op/fork"
	"github.com/brimdata/super/runtime/sam/op/fuse"
	"github.com/brimdata/super/runtime/sam/op/head"
	"github.com/brimdata/super/runtime/sam/op/join"
	"github.com/brimdata/super/runtime/sam/op/load"
	"github.com/brimdata/super/runtime/sam/op/merge"
	"github.com/brimdata/super/runtime/sam/op/meta"
	"github.com/brimdata/super/runtime/sam/op/mirror"
	"github.com/brimdata/super/runtime/sam/op/robot"
	"github.com/brimdata/super/runtime/sam/op/shape"
	"github.com/brimdata/super/runtime/sam/op/sort"
	"github.com/brimdata/super/runtime/sam/op/switcher"
	"github.com/brimdata/super/runtime/sam/op/tail"
	"github.com/brimdata/super/runtime/sam/op/top"
	"github.com/brimdata/super/runtime/sam/op/traverse"
	"github.com/brimdata/super/runtime/sam/op/uniq"
	"github.com/brimdata/super/runtime/sam/op/yield"
	"github.com/brimdata/super/runtime/vam"
	vamop "github.com/brimdata/super/runtime/vam/op"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/segmentio/ksuid"
)

var ErrJoinParents = errors.New("join requires two upstream parallel query paths")

type Builder struct {
	rctx         *runtime.Context
	mctx         *super.Context
	env          *exec.Environment
	readers      []zio.Reader
	progress     *zbuf.Progress
	channels     map[string][]zbuf.Puller
	deletes      *sync.Map
	udfs         map[string]dag.Expr
	compiledUDFs map[string]*expr.UDF
	resetters    expr.Resetters
}

func NewBuilder(rctx *runtime.Context, env *exec.Environment) *Builder {
	return &Builder{
		rctx: rctx,
		mctx: super.NewContext(),
		env:  env,
		progress: &zbuf.Progress{
			BytesRead:      0,
			BytesMatched:   0,
			RecordsRead:    0,
			RecordsMatched: 0,
		},
		channels:     make(map[string][]zbuf.Puller),
		udfs:         make(map[string]dag.Expr),
		compiledUDFs: make(map[string]*expr.UDF),
	}
}

// Build builds a flowgraph for seq.  If seq contains a dag.DefaultSource, it
// will read from readers.
func (b *Builder) Build(seq dag.Seq, readers ...zio.Reader) (map[string]zbuf.Puller, error) {
	if !isEntry(seq) {
		return nil, errors.New("internal error: DAG entry point is not a data source")
	}
	b.readers = readers
	if b.env.UseVAM() {
		if _, err := b.compileVamSeq(seq, nil); err != nil {
			return nil, err
		}
	} else {
		if _, err := b.compileSeq(seq, nil); err != nil {
			return nil, err
		}
	}
	channels := make(map[string]zbuf.Puller)
	for key, pullers := range b.channels {
		if len(pullers) == 1 {
			channels[key] = pullers[0]
		} else {
			channels[key] = combine.New(b.rctx, pullers)
		}
	}
	return channels, nil
}

func (b *Builder) BuildWithPuller(seq dag.Seq, parent vector.Puller) ([]vector.Puller, error) {
	return b.compileVamSeq(seq, []vector.Puller{parent})
}

func (b *Builder) BuildVamToSeqFilter(filter dag.Expr, poolID, commitID ksuid.KSUID) (zbuf.Puller, error) {
	pool, err := b.env.Lake().OpenPool(b.rctx.Context, poolID)
	if err != nil {
		return nil, err
	}
	e, err := b.compileVamExpr(filter)
	if err != nil {
		return nil, err
	}
	l, err := meta.NewSortedLister(b.rctx.Context, b.mctx, pool, commitID, nil)
	if err != nil {
		return nil, err
	}
	cache := b.env.Lake().VectorCache()
	project, _ := optimizer.FieldsOf(filter)
	search, err := vamop.NewSearcher(b.rctx, cache, l, pool, e, project)
	if err != nil {
		return nil, err
	}
	return meta.NewSearchScanner(b.rctx, search, pool, b.newPushdown(filter, nil), b.progress), nil
}

func (b *Builder) sctx() *super.Context {
	return b.rctx.Sctx
}

func (b *Builder) Meter() zbuf.Meter {
	return b.progress
}

func (b *Builder) Deletes() *sync.Map {
	return b.deletes
}

func (b *Builder) resetResetters() {
	b.resetters = nil
}

func (b *Builder) compileLeaf(o dag.Op, parent zbuf.Puller) (zbuf.Puller, error) {
	switch v := o.(type) {
	case *dag.Aggregate:
		return b.compileAggregate(parent, v)
	case *dag.Cut:
		b.resetResetters()
		assignments, err := b.compileAssignments(v.Args)
		if err != nil {
			return nil, err
		}
		lhs, rhs := splitAssignments(assignments)
		cutter := expr.NewCutter(b.sctx(), lhs, rhs)
		return op.NewApplier(b.rctx, parent, cutter, b.resetters), nil
	case *dag.Drop:
		fields := make(field.List, 0, len(v.Args))
		for _, e := range v.Args {
			fields = append(fields, e.(*dag.This).Path)
		}
		dropper := expr.NewDropper(b.sctx(), fields)
		return op.NewApplier(b.rctx, parent, dropper, expr.Resetters{}), nil
	case *dag.Distinct:
		b.resetResetters()
		e, err := b.compileExpr(v.Expr)
		if err != nil {
			return nil, err
		}
		return distinct.New(parent, e), nil
	case *dag.Sort:
		b.resetResetters()
		var sortExprs []expr.SortExpr
		for _, s := range v.Args {
			k, err := b.compileExpr(s.Key)
			if err != nil {
				return nil, err
			}
			sortExprs = append(sortExprs, expr.NewSortExpr(k, s.Order, order.NullsLast))
		}
		return sort.New(b.rctx, parent, sortExprs, v.NullsFirst, v.Reverse, b.resetters), nil
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
		return uniq.New(b.rctx, parent, v.Cflag), nil
	case *dag.Pass:
		return parent, nil
	case *dag.Filter:
		b.resetResetters()
		f, err := b.compileExpr(v.Expr)
		if err != nil {
			return nil, fmt.Errorf("compiling filter: %w", err)
		}
		return op.NewApplier(b.rctx, parent, expr.NewFilterApplier(b.sctx(), f), b.resetters), nil
	case *dag.Top:
		b.resetResetters()
		var sortExprs []expr.SortExpr
		for _, dagSortExpr := range v.Exprs {
			e, err := b.compileExpr(dagSortExpr.Key)
			if err != nil {
				return nil, err
			}
			sortExprs = append(sortExprs, expr.NewSortExpr(e, dagSortExpr.Order, order.NullsLast))
		}
		return top.New(b.sctx(), parent, v.Limit, sortExprs, v.NullsFirst, v.Reverse, b.resetters), nil
	case *dag.Put:
		b.resetResetters()
		clauses, err := b.compileAssignments(v.Args)
		if err != nil {
			return nil, err
		}
		putter := expr.NewPutter(b.sctx(), clauses)
		return op.NewApplier(b.rctx, parent, putter, b.resetters), nil
	case *dag.Rename:
		b.resetResetters()
		srcs, dsts, err := b.compileAssignmentsToLvals(v.Args)
		if err != nil {
			return nil, err
		}
		renamer := expr.NewRenamer(b.sctx(), srcs, dsts)
		return op.NewApplier(b.rctx, parent, renamer, b.resetters), nil
	case *dag.Fuse:
		return fuse.New(b.rctx, parent)
	case *dag.Shape:
		return shape.New(b.rctx, parent)
	case *dag.Join:
		return nil, ErrJoinParents
	case *dag.Merge:
		return nil, errors.New("merge: multiple upstream paths required")
	case *dag.Explode:
		typ, err := sup.ParseType(b.sctx(), v.Type)
		if err != nil {
			return nil, err
		}
		b.resetResetters()
		args, err := b.compileExprs(v.Args)
		if err != nil {
			return nil, err
		}
		return explode.New(b.sctx(), parent, args, typ, v.As, b.resetters)
	case *dag.Over:
		return b.compileOver(parent, v)
	case *dag.Yield:
		b.resetResetters()
		exprs, err := b.compileExprs(v.Exprs)
		if err != nil {
			return nil, err
		}
		t := yield.New(parent, exprs, b.resetters)
		return t, nil
	case *dag.PoolScan:
		if parent != nil {
			return nil, errors.New("internal error: pool scan cannot have a parent operator")
		}
		return b.compilePoolScan(v)
	case *dag.PoolMetaScan:
		return meta.NewPoolMetaScanner(b.rctx.Context, b.sctx(), b.env.Lake(), v.ID, v.Meta)
	case *dag.CommitMetaScan:
		var pruner expr.Evaluator
		if v.Tap && v.KeyPruner != nil {
			var err error
			pruner, err = compileExpr(v.KeyPruner)
			if err != nil {
				return nil, err
			}
		}
		return meta.NewCommitMetaScanner(b.rctx.Context, b.sctx(), b.env.Lake(), v.Pool, v.Commit, v.Meta, pruner)
	case *dag.LakeMetaScan:
		return meta.NewLakeMetaScanner(b.rctx.Context, b.sctx(), b.env.Lake(), v.Meta)
	case *dag.HTTPScan:
		body := strings.NewReader(v.Body)
		return b.env.OpenHTTP(b.rctx.Context, b.sctx(), v.URL, v.Format, v.Method, v.Headers, body, nil)
	case *dag.FileScan:
		var dataFilter dag.Expr
		if v.Pushdown.DataFilter != nil {
			dataFilter = v.Pushdown.DataFilter.Expr
		}
		return b.env.Open(b.rctx.Context, b.sctx(), v.Path, v.Format, b.newPushdown(dataFilter, v.Pushdown.Projection))
	case *dag.RobotScan:
		e, err := compileExpr(v.Expr)
		if err != nil {
			return nil, err
		}
		return robot.New(b.rctx, b.env, parent, e, v.Format, b.newPushdown(v.Filter, nil)), nil
	case *dag.DefaultScan:
		pushdown := b.newPushdown(v.Filter, nil)
		if len(b.readers) == 1 {
			return zbuf.NewScanner(b.rctx.Context, b.readers[0], pushdown)
		}
		scanners := make([]zbuf.Scanner, 0, len(b.readers))
		for _, r := range b.readers {
			scanner, err := zbuf.NewScanner(b.rctx.Context, r, pushdown)
			if err != nil {
				return nil, err
			}
			scanners = append(scanners, scanner)
		}
		return zbuf.MultiScanner(scanners...), nil
	case *dag.NullScan:
		//XXX we need something that implements the done protocol and restarst
		return zbuf.NewPuller(zbuf.NewArray([]super.Value{super.Null})), nil
	case *dag.Lister:
		if parent != nil {
			return nil, errors.New("internal error: data source cannot have a parent operator")
		}
		pool, err := b.lookupPool(v.Pool)
		if err != nil {
			return nil, err
		}
		var pruner expr.Evaluator
		if v.KeyPruner != nil {
			pruner, err = compileExpr(v.KeyPruner)
			if err != nil {
				return nil, err
			}
		}
		return meta.NewSortedLister(b.rctx.Context, b.mctx, pool, v.Commit, pruner)
	case *dag.Slicer:
		return meta.NewSlicer(parent, b.mctx), nil
	case *dag.SeqScan:
		pool, err := b.lookupPool(v.Pool)
		if err != nil {
			return nil, err
		}
		var pruner expr.Evaluator
		if v.KeyPruner != nil {
			pruner, err = compileExpr(v.KeyPruner)
			if err != nil {
				return nil, err
			}
		}
		return meta.NewSequenceScanner(b.rctx, parent, pool, b.newPushdown(v.Filter, nil), pruner, b.progress), nil
	case *dag.Deleter:
		pool, err := b.lookupPool(v.Pool)
		if err != nil {
			return nil, err
		}
		var pruner expr.Evaluator
		if v.KeyPruner != nil {
			pruner, err = compileExpr(v.KeyPruner)
			if err != nil {
				return nil, err
			}
		}
		if b.deletes == nil {
			b.deletes = &sync.Map{}
		}
		var filter *deleter
		if f := b.newPushdown(v.Where, nil); f != nil {
			filter = &deleter{*f}
		}
		return meta.NewDeleter(b.rctx, parent, pool, filter, pruner, b.progress, b.deletes), nil
	case *dag.Load:
		return load.New(b.rctx, b.env.Lake(), parent, v.Pool, v.Branch, v.Author, v.Message, v.Meta), nil
	case *dag.Vectorize:
		// If the first op is SeqScan, then pull it out so we can
		// give the scanner a zio.Puller parent (i.e., the lister).
		if scan, ok := v.Body[0].(*dag.SeqScan); ok {
			puller, err := b.compileVamScan(scan, parent)
			if err != nil {
				return nil, err
			}
			if len(v.Body) > 1 {
				outputs, err := b.compileVamSeq(v.Body[1:], []vector.Puller{puller})
				if err != nil {
					return nil, err
				}
				if len(outputs) == 1 {
					puller = outputs[0]
				} else {
					puller = vamop.NewCombine(b.rctx, outputs)
				}
			}
			return vam.NewMaterializer(puller), nil
		}
		//XXX
		return nil, errors.New("dag.Vectorize must begin with SeqScan")
	case *dag.Output:
		b.channels[v.Name] = append(b.channels[v.Name], parent)
		return parent, nil
	default:
		return nil, fmt.Errorf("unknown DAG operator type: %v", v)
	}
}

func (b *Builder) compileDefs(defs []dag.Def) ([]string, []expr.Evaluator, error) {
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

func (b *Builder) compileOver(parent zbuf.Puller, over *dag.Over) (zbuf.Puller, error) {
	b.resetResetters()
	withNames, withExprs, err := b.compileDefs(over.Defs)
	if err != nil {
		return nil, err
	}
	exprs, err := b.compileExprs(over.Exprs)
	if err != nil {
		return nil, err
	}
	enter := traverse.NewOver(b.rctx, parent, exprs, b.resetters)
	if over.Body == nil {
		return enter, nil
	}
	scope := enter.AddScope(b.rctx.Context, withNames, withExprs)
	exits, err := b.compileSeq(over.Body, []zbuf.Puller{scope})
	if err != nil {
		return nil, err
	}
	var exit zbuf.Puller
	if len(exits) == 1 {
		exit = exits[0]
	} else {
		// This can happen when output of over body
		// is a fork or switch.
		exit = combine.New(b.rctx, exits)
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

func (b *Builder) compileAssignmentsToLvals(assignments []dag.Assignment) ([]*expr.Lval, []*expr.Lval, error) {
	var srcs, dsts []*expr.Lval
	for _, a := range assignments {
		src, err := b.compileLval(a.RHS)
		if err != nil {
			return nil, nil, err
		}
		dst, err := b.compileLval(a.LHS)
		if err != nil {
			return nil, nil, err
		}
		srcs = append(srcs, src)
		dsts = append(dsts, dst)
	}
	return srcs, dsts, nil
}

func splitAssignments(assignments []expr.Assignment) ([]*expr.Lval, []expr.Evaluator) {
	n := len(assignments)
	lhs := make([]*expr.Lval, 0, n)
	rhs := make([]expr.Evaluator, 0, n)
	for _, a := range assignments {
		lhs = append(lhs, a.LHS)
		rhs = append(rhs, a.RHS)
	}
	return lhs, rhs
}

func (b *Builder) compileSeq(seq dag.Seq, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	for _, o := range seq {
		var err error
		parents, err = b.compile(o, parents)
		if err != nil {
			return nil, err
		}
	}
	return parents, nil
}

func (b *Builder) compileScope(scope *dag.Scope, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	// Because there can be name collisions between a child and parent scope
	// we clone the current udf map, populate the cloned map, then restore the
	// old scope once the current scope has been built.
	parentUDFs := b.udfs
	b.udfs = maps.Clone(parentUDFs)
	defer func() { b.udfs = parentUDFs }()
	for _, f := range scope.Funcs {
		b.udfs[f.Name] = f.Expr
	}
	return b.compileSeq(scope.Body, parents)
}

func (b *Builder) compileFork(par *dag.Fork, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	var f *fork.Op
	switch len(parents) {
	case 0:
		// No parents: no need for a fork since every op gets a nil parent.
	case 1:
		// Single parent: insert a fork for n-way fanout.
		f = fork.New(b.rctx, parents[0])
	default:
		// Multiple parents: insert a combine followed by a fork for n-way fanout.
		f = fork.New(b.rctx, combine.New(b.rctx, parents))
	}
	var ops []zbuf.Puller
	for _, seq := range par.Paths {
		var parent zbuf.Puller
		if f != nil && !isEntry(seq) {
			parent = f.AddExit()
		}
		op, err := b.compileSeq(seq, []zbuf.Puller{parent})
		if err != nil {
			return nil, err
		}
		ops = append(ops, op...)
	}
	return ops, nil
}

func (b *Builder) compileScatter(par *dag.Scatter, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	if len(parents) != 1 {
		return nil, errors.New("internal error: scatter operator requires a single parent")
	}
	var ops []zbuf.Puller
	for _, o := range par.Paths {
		op, err := b.compileSeq(o, parents[:1])
		if err != nil {
			return nil, err
		}
		ops = append(ops, op...)
	}
	return ops, nil
}

func (b *Builder) compileMirror(m *dag.Mirror, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	parent := parents[0]
	if len(parents) > 1 {
		parent = combine.New(b.rctx, parents)
	}
	o := mirror.New(b.rctx, parent)
	main, err := b.compileSeq(m.Main, []zbuf.Puller{o})
	if err != nil {
		return nil, err
	}
	mirrored, err := b.compileSeq(m.Mirror, []zbuf.Puller{o.Mirrored()})
	if err != nil {
		return nil, err
	}
	return append(main, mirrored...), nil
}

func (b *Builder) compileExprSwitch(swtch *dag.Switch, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	parent := parents[0]
	if len(parents) > 1 {
		parent = combine.New(b.rctx, parents)
	}
	b.resetResetters()
	e, err := b.compileExpr(swtch.Expr)
	if err != nil {
		return nil, err
	}
	s := exprswitch.New(b.rctx, parent, e, b.resetters)
	var exits []zbuf.Puller
	for _, c := range swtch.Cases {
		var val *super.Value
		if c.Expr != nil {
			val2, err := b.evalAtCompileTime(c.Expr)
			if err != nil {
				return nil, err
			}
			if val2.IsError() {
				return nil, errors.New("switch case is not a constant expression")
			}
			val = &val2
		}
		parents, err := b.compileSeq(c.Path, []zbuf.Puller{s.AddCase(val)})
		if err != nil {
			return nil, err
		}
		exits = append(exits, parents...)
	}
	return exits, nil
}

func (b *Builder) compileSwitch(swtch *dag.Switch, parents []zbuf.Puller) ([]zbuf.Puller, error) {
	parent := parents[0]
	if len(parents) > 1 {
		parent = combine.New(b.rctx, parents)
	}
	b.resetResetters()
	var exprs []expr.Evaluator
	for _, c := range swtch.Cases {
		e, err := b.compileExpr(c.Expr)
		if err != nil {
			return nil, fmt.Errorf("compiling switch case filter: %w", err)
		}
		exprs = append(exprs, e)
	}
	switcher := switcher.New(b.rctx, parent, b.resetters)
	var ops []zbuf.Puller
	for i, e := range exprs {
		o, err := b.compileSeq(swtch.Cases[i].Path, []zbuf.Puller{switcher.AddCase(e)})
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
	case *dag.Fork:
		return b.compileFork(o, parents)
	case *dag.Scatter:
		return b.compileScatter(o, parents)
	case *dag.Mirror:
		return b.compileMirror(o, parents)
	case *dag.Scope:
		return b.compileScope(o, parents)
	case *dag.Switch:
		if o.Expr != nil {
			return b.compileExprSwitch(o, parents)
		}
		return b.compileSwitch(o, parents)
	case *dag.Join:
		if len(parents) != 2 {
			return nil, ErrJoinParents
		}
		b.resetResetters()
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
		leftDir, rightDir := o.LeftDir, o.RightDir
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
			leftDir, rightDir = rightDir, leftDir
		default:
			return nil, fmt.Errorf("unknown kind of join: '%s'", o.Style)
		}
		join := join.New(b.rctx, anti, inner, leftParent, rightParent, leftKey, rightKey, leftDir, rightDir, lhs, rhs, b.resetters)
		return []zbuf.Puller{join}, nil
	case *dag.Merge:
		b.resetResetters()
		e, err := b.compileExpr(o.Expr)
		if err != nil {
			return nil, err
		}
		cmp := expr.NewComparator(expr.NewSortExpr(e, o.Order, o.Order.NullsMax(true))).WithMissingAsNull()
		return []zbuf.Puller{merge.New(b.rctx, parents, cmp.Compare, b.resetters)}, nil
	case *dag.Combine:
		return []zbuf.Puller{combine.New(b.rctx, parents)}, nil
	default:
		var parent zbuf.Puller
		if len(parents) == 1 {
			parent = parents[0]
		} else if len(parents) > 1 {
			parent = combine.New(b.rctx, parents)
		}
		p, err := b.compileLeaf(o, parent)
		if err != nil {
			return nil, err
		}
		return []zbuf.Puller{p}, nil
	}
}

func (b *Builder) compilePoolScan(scan *dag.PoolScan) (zbuf.Puller, error) {
	// Here we convert PoolScan to lister->slicer->seqscan for the slow path as
	// optimizer should do this conversion, but this allows us to run
	// unoptimized scans too.
	pool, err := b.lookupPool(scan.ID)
	if err != nil {
		return nil, err
	}
	l, err := meta.NewSortedLister(b.rctx.Context, b.mctx, pool, scan.Commit, nil)
	if err != nil {
		return nil, err
	}
	slicer := meta.NewSlicer(l, b.mctx)
	return meta.NewSequenceScanner(b.rctx, slicer, pool, nil, nil, b.progress), nil
}

// For runtime/sam/expr/filter_test.go
func NewPushdown(b *Builder, e dag.Expr) zbuf.Pushdown {
	return b.newPushdown(e, nil)
}

func (b *Builder) newPushdown(e dag.Expr, projection []field.Path) *pushdown {
	if e == nil && projection == nil {
		return nil
	}
	return &pushdown{
		dataFilter: e,
		builder:    b,
		projection: field.NewProjection(projection),
	}
}

func (b *Builder) newMetaPushdown(e dag.Expr, projection, metaProjection []field.Path) *pushdown {
	return &pushdown{
		metaFilter:     e,
		builder:        b,
		projection:     field.NewProjection(projection),
		metaProjection: field.NewProjection(metaProjection),
	}
}

func (b *Builder) lookupPool(id ksuid.KSUID) (*lake.Pool, error) {
	if b.env == nil || b.env.Lake() == nil {
		return nil, errors.New("internal error: lake operation cannot be used in non-lake context")
	}
	// This is fast because of the pool cache in the lake.
	return b.env.Lake().OpenPool(b.rctx.Context, id)
}

func (b *Builder) evalAtCompileTime(in dag.Expr) (val super.Value, err error) {
	if in == nil {
		return super.Null, nil
	}
	e, err := b.compileExpr(in)
	if err != nil {
		return super.Null, err
	}
	// Catch panic as the runtime will panic if there is a
	// reference to a var not in scope, a field access null this, etc.
	defer func() {
		if recover() != nil {
			val = b.sctx().Missing()
		}
	}()
	return e.Eval(expr.NewContext(), b.sctx().Missing()), nil
}

func compileExpr(in dag.Expr) (expr.Evaluator, error) {
	b := NewBuilder(runtime.NewContext(context.Background(), super.NewContext()), nil)
	return b.compileExpr(in)
}

func EvalAtCompileTime(sctx *super.Context, in dag.Expr) (val super.Value, err error) {
	// We pass in a nil adaptor, which causes a panic for anything adaptor
	// related, which is not currently allowed in an expression sub-query.
	b := NewBuilder(runtime.NewContext(context.Background(), sctx), nil)
	return b.evalAtCompileTime(in)
}

func isEntry(seq dag.Seq) bool {
	if len(seq) == 0 {
		return false
	}
	switch op := seq[0].(type) {
	case *dag.Lister, *dag.DefaultScan, *dag.FileScan, *dag.HTTPScan, *dag.PoolScan, *dag.LakeMetaScan, *dag.PoolMetaScan, *dag.CommitMetaScan, *dag.NullScan:
		return true
	case *dag.Scope:
		return isEntry(op.Body)
	case *dag.Fork:
		return len(op.Paths) > 0 && !slices.ContainsFunc(op.Paths, func(seq dag.Seq) bool {
			return !isEntry(seq)
		})
	}
	return false
}
