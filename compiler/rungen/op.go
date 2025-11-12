package rungen

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/optimizer"
	"github.com/brimdata/super/db"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/runtime/sam/op/combine"
	"github.com/brimdata/super/runtime/sam/op/count"
	"github.com/brimdata/super/runtime/sam/op/distinct"
	"github.com/brimdata/super/runtime/sam/op/explode"
	"github.com/brimdata/super/runtime/sam/op/exprswitch"
	"github.com/brimdata/super/runtime/sam/op/filescan"
	"github.com/brimdata/super/runtime/sam/op/fork"
	"github.com/brimdata/super/runtime/sam/op/fuse"
	"github.com/brimdata/super/runtime/sam/op/head"
	"github.com/brimdata/super/runtime/sam/op/load"
	"github.com/brimdata/super/runtime/sam/op/merge"
	"github.com/brimdata/super/runtime/sam/op/meta"
	"github.com/brimdata/super/runtime/sam/op/mirror"
	"github.com/brimdata/super/runtime/sam/op/robot"
	"github.com/brimdata/super/runtime/sam/op/scope"
	"github.com/brimdata/super/runtime/sam/op/skip"
	"github.com/brimdata/super/runtime/sam/op/sort"
	"github.com/brimdata/super/runtime/sam/op/switcher"
	"github.com/brimdata/super/runtime/sam/op/tail"
	"github.com/brimdata/super/runtime/sam/op/top"
	"github.com/brimdata/super/runtime/sam/op/uniq"
	"github.com/brimdata/super/runtime/sam/op/unnest"
	"github.com/brimdata/super/runtime/sam/op/values"
	"github.com/brimdata/super/runtime/vam"
	vamexpr "github.com/brimdata/super/runtime/vam/expr"
	vamop "github.com/brimdata/super/runtime/vam/op"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/segmentio/ksuid"
)

var ErrJoinParents = errors.New("join requires two upstream parallel query paths")

type Builder struct {
	rctx            *runtime.Context
	mctx            *super.Context
	env             *exec.Environment
	readers         []sio.Reader
	progress        *sbuf.Progress
	channels        map[string][]sbuf.Puller
	deletes         *sync.Map
	funcs           map[string]*dag.FuncDef
	compiledUDFs    map[string]*expr.UDF
	compiledVamUDFs map[string]*vamexpr.UDF
}

func NewBuilder(rctx *runtime.Context, env *exec.Environment) *Builder {
	return &Builder{
		rctx: rctx,
		mctx: super.NewContext(),
		env:  env,
		progress: &sbuf.Progress{
			BytesRead:      0,
			BytesMatched:   0,
			RecordsRead:    0,
			RecordsMatched: 0,
		},
		channels:        make(map[string][]sbuf.Puller),
		funcs:           make(map[string]*dag.FuncDef),
		compiledUDFs:    make(map[string]*expr.UDF),
		compiledVamUDFs: make(map[string]*vamexpr.UDF),
	}
}

// Build builds a flowgraph for main.  If main contains a dag.DefaultSource, it
// will read from readers.
func (b *Builder) Build(main *dag.Main, readers ...sio.Reader) (map[string]sbuf.Puller, error) {
	if !isEntry(main.Body) {
		return nil, errors.New("internal error: DAG entry point is not a data source")
	}
	b.readers = readers
	if b.env.UseVAM() {
		if _, err := b.compileVamMain(main, nil); err != nil {
			return nil, err
		}
	} else {
		if _, err := b.compileMain(main, nil); err != nil {
			return nil, err
		}
	}
	channels := make(map[string]sbuf.Puller)
	for key, pullers := range b.channels {
		channels[key] = b.combine(pullers)
	}
	return channels, nil
}

func (b *Builder) BuildWithPuller(seq dag.Seq, parent vector.Puller) ([]vector.Puller, error) {
	return b.compileVamSeq(seq, []vector.Puller{parent})
}

func (b *Builder) BuildVamToSeqFilter(filter dag.Expr, poolID, commitID ksuid.KSUID) (sbuf.Puller, error) {
	pool, err := b.env.DB().OpenPool(b.rctx.Context, poolID)
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
	cache := b.env.DB().VectorCache()
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

func (b *Builder) Meter() sbuf.Meter {
	return b.progress
}

func (b *Builder) Deletes() *sync.Map {
	return b.deletes
}

func (b *Builder) compileMain(main *dag.Main, parents []sbuf.Puller) ([]sbuf.Puller, error) {
	for _, f := range main.Funcs {
		b.funcs[f.Tag] = f
	}
	return b.compileSeq(main.Body, parents)
}

func (b *Builder) compileLeaf(o dag.Op, parent sbuf.Puller) (sbuf.Puller, error) {
	switch v := o.(type) {
	//
	// Scanners in alphatbetical order.
	//
	case *dag.CommitMetaScan:
		var pruner expr.Evaluator
		if v.Tap && v.KeyPruner != nil {
			var err error
			pruner, err = compileExpr(v.KeyPruner)
			if err != nil {
				return nil, err
			}
		}
		return meta.NewCommitMetaScanner(b.rctx.Context, b.sctx(), b.env.DB(), v.Pool, v.Commit, v.Meta, pruner)
	case *dag.DBMetaScan:
		return meta.NewDBMetaScanner(b.rctx.Context, b.sctx(), b.env.DB(), v.Meta)
	case *dag.DefaultScan:
		pushdown := b.newPushdown(v.Filter, nil)
		if len(b.readers) == 1 {
			return sbuf.NewScanner(b.rctx.Context, b.readers[0], pushdown)
		}
		scanners := make([]sbuf.Scanner, 0, len(b.readers))
		for _, r := range b.readers {
			scanner, err := sbuf.NewScanner(b.rctx.Context, r, pushdown)
			if err != nil {
				return nil, err
			}
			scanners = append(scanners, scanner)
		}
		return sbuf.MultiScanner(scanners...), nil
	case *dag.DeleterScan:
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
		pushdown := b.newPushdown(v.Where, nil)
		if pushdown != nil {
			pushdown = &deleter{pushdown, b, v.Where}
		}
		return meta.NewDeleter(b.rctx, parent, pool, pushdown, pruner, b.progress, b.deletes), nil
	case *dag.FileScan:
		var dataFilter dag.Expr
		if v.Pushdown.DataFilter != nil {
			dataFilter = v.Pushdown.DataFilter.Expr
		}
		pushdown := b.newPushdown(dataFilter, v.Pushdown.Projection)
		return filescan.New(b.rctx, b.env, v.Paths, v.Format, pushdown), nil
	case *dag.HTTPScan:
		body := strings.NewReader(v.Body)
		return b.env.OpenHTTP(b.rctx.Context, b.sctx(), v.URL, v.Format, v.Method, v.Headers, body, nil)
	case *dag.ListerScan:
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
	case *dag.NullScan:
		return sbuf.NewPuller(sbuf.NewArray([]super.Value{super.Null})), nil
	case *dag.PoolMetaScan:
		return meta.NewPoolMetaScanner(b.rctx.Context, b.sctx(), b.env.DB(), v.ID, v.Meta)
	case *dag.PoolScan:
		if parent != nil {
			return nil, errors.New("internal error: pool scan cannot have a parent operator")
		}
		return b.compilePoolScan(v)
	case *dag.RobotScan:
		e, err := compileExpr(v.Expr)
		if err != nil {
			return nil, err
		}
		return robot.New(b.rctx, b.env, parent, e, v.Format, b.newPushdown(v.Filter, nil)), nil
	case *dag.SlicerOp:
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
	//
	// Non-scanner operators in alphabetical order.
	//
	case *dag.AggregateOp:
		return b.compileAggregate(parent, v)
	case *dag.CountOp:
		var e expr.Evaluator
		if v.Expr != nil {
			var err error
			if e, err = b.compileExpr(v.Expr); err != nil {
				return nil, err
			}
		}
		return count.New(b.rctx.Sctx, parent, v.Alias, e)
	case *dag.CutOp:
		assignments, err := b.compileAssignments(v.Args)
		if err != nil {
			return nil, err
		}
		lhs, rhs := splitAssignments(assignments)
		cutter := expr.NewCutter(b.sctx(), lhs, rhs)
		return op.NewApplier(b.rctx, parent, cutter), nil
	case *dag.DropOp:
		fields := make(field.List, 0, len(v.Args))
		for _, e := range v.Args {
			fields = append(fields, e.(*dag.ThisExpr).Path)
		}
		dropper := expr.NewDropper(b.sctx(), fields)
		return op.NewApplier(b.rctx, parent, dropper), nil
	case *dag.DistinctOp:
		e, err := b.compileExpr(v.Expr)
		if err != nil {
			return nil, err
		}
		return distinct.New(parent, e), nil
	case *dag.ExplodeOp:
		typ, err := sup.ParseType(b.sctx(), v.Type)
		if err != nil {
			return nil, err
		}
		args, err := b.compileExprs(v.Args)
		if err != nil {
			return nil, err
		}
		return explode.New(b.sctx(), parent, args, typ, v.As)
	case *dag.FilterOp:
		f, err := b.compileExpr(v.Expr)
		if err != nil {
			return nil, fmt.Errorf("compiling filter: %w", err)
		}
		return op.NewApplier(b.rctx, parent, expr.NewFilterApplier(b.sctx(), f)), nil
	case *dag.FuseOp:
		return fuse.New(b.rctx, parent)
	case *dag.HashJoinOp, *dag.JoinOp:
		return nil, ErrJoinParents
	case *dag.MergeOp:
		return nil, errors.New("merge: multiple upstream paths required")
	case *dag.HeadOp:
		return head.New(parent, v.Count), nil
	case *dag.LoadOp:
		return load.New(b.rctx, b.env.DB(), parent, v.Pool, v.Branch, v.Author, v.Message, v.Meta), nil
	case *dag.OutputOp:
		b.channels[v.Name] = append(b.channels[v.Name], parent)
		return parent, nil
	case *dag.PassOp:
		return parent, nil
	case *dag.PutOp:
		clauses, err := b.compileAssignments(v.Args)
		if err != nil {
			return nil, err
		}
		putter := expr.NewPutter(b.sctx(), clauses)
		return op.NewApplier(b.rctx, parent, putter), nil
	case *dag.RenameOp:
		srcs, dsts, err := b.compileAssignmentsToLvals(v.Args)
		if err != nil {
			return nil, err
		}
		renamer := expr.NewRenamer(b.sctx(), srcs, dsts)
		return op.NewApplier(b.rctx, parent, renamer), nil
	case *dag.SkipOp:
		return skip.New(parent, v.Count), nil
	case *dag.SortOp:
		var sortExprs []expr.SortExpr
		for _, e := range v.Exprs {
			k, err := b.compileExpr(e.Key)
			if err != nil {
				return nil, err
			}
			sortExprs = append(sortExprs, expr.NewSortExpr(k, e.Order, e.Nulls))
		}
		return sort.New(b.rctx, parent, sortExprs, v.Reverse), nil
	case *dag.TailOp:
		return tail.New(parent, v.Count), nil
	case *dag.TopOp:
		var sortExprs []expr.SortExpr
		for _, dagSortExpr := range v.Exprs {
			e, err := b.compileExpr(dagSortExpr.Key)
			if err != nil {
				return nil, err
			}
			sortExprs = append(sortExprs, expr.NewSortExpr(e, dagSortExpr.Order, dagSortExpr.Nulls))
		}
		return top.New(b.sctx(), parent, v.Limit, sortExprs, v.Reverse), nil
	case *dag.UniqOp:
		return uniq.New(b.rctx, parent, v.Cflag), nil
	case *dag.UnnestOp:
		return b.compileUnnest(parent, v)
	case *dag.ValuesOp:
		exprs, err := b.compileExprs(v.Exprs)
		if err != nil {
			return nil, err
		}
		t := values.New(parent, exprs)
		return t, nil

	default:
		return nil, fmt.Errorf("unknown DAG operator type: %v", v)
	}
}

func (b *Builder) compileUnnest(parent sbuf.Puller, u *dag.UnnestOp) (sbuf.Puller, error) {
	expr, err := b.compileExpr(u.Expr)
	if err != nil {
		return nil, err
	}
	unnestOp := unnest.NewUnnest(b.rctx, parent, expr)
	if u.Body == nil {
		return unnestOp, nil
	}
	scope := scope.NewScope(b.rctx.Context, unnestOp)
	exits, err := b.compileSeq(u.Body, []sbuf.Puller{scope})
	if err != nil {
		return nil, err
	}
	return scope.NewExit(b.combine(exits)), nil
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

func (b *Builder) compileSeq(seq dag.Seq, parents []sbuf.Puller) ([]sbuf.Puller, error) {
	for _, o := range seq {
		var err error
		parents, err = b.compile(o, parents)
		if err != nil {
			return nil, err
		}
	}
	return parents, nil
}

func (b *Builder) compileFork(par *dag.ForkOp, parents []sbuf.Puller) ([]sbuf.Puller, error) {
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
	var ops []sbuf.Puller
	for _, seq := range par.Paths {
		var parent sbuf.Puller
		if f != nil && !isEntry(seq) {
			parent = f.AddExit()
		}
		op, err := b.compileSeq(seq, []sbuf.Puller{parent})
		if err != nil {
			return nil, err
		}
		ops = append(ops, op...)
	}
	return ops, nil
}

func (b *Builder) compileScatter(par *dag.ScatterOp, parents []sbuf.Puller) ([]sbuf.Puller, error) {
	if len(parents) != 1 {
		return nil, errors.New("internal error: scatter operator requires a single parent")
	}
	var ops []sbuf.Puller
	for _, o := range par.Paths {
		op, err := b.compileSeq(o, parents[:1])
		if err != nil {
			return nil, err
		}
		ops = append(ops, op...)
	}
	return ops, nil
}

func (b *Builder) compileMirror(m *dag.MirrorOp, parents []sbuf.Puller) ([]sbuf.Puller, error) {
	o := mirror.New(b.rctx, b.combine(parents))
	main, err := b.compileSeq(m.Main, []sbuf.Puller{o})
	if err != nil {
		return nil, err
	}
	mirrored, err := b.compileSeq(m.Mirror, []sbuf.Puller{o.Mirrored()})
	if err != nil {
		return nil, err
	}
	return append(main, mirrored...), nil
}

func (b *Builder) compileExprSwitch(swtch *dag.SwitchOp, parents []sbuf.Puller) ([]sbuf.Puller, error) {
	e, err := b.compileExpr(swtch.Expr)
	if err != nil {
		return nil, err
	}
	s := exprswitch.New(b.rctx, b.combine(parents), e)
	var exits []sbuf.Puller
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
		parents, err := b.compileSeq(c.Path, []sbuf.Puller{s.AddCase(val)})
		if err != nil {
			return nil, err
		}
		exits = append(exits, parents...)
	}
	return exits, nil
}

func (b *Builder) compileSwitch(swtch *dag.SwitchOp, parents []sbuf.Puller) ([]sbuf.Puller, error) {
	var exprs []expr.Evaluator
	for _, c := range swtch.Cases {
		e, err := b.compileExpr(c.Expr)
		if err != nil {
			return nil, fmt.Errorf("compiling switch case filter: %w", err)
		}
		exprs = append(exprs, e)
	}
	switcher := switcher.New(b.rctx, b.combine(parents))
	var ops []sbuf.Puller
	for i, e := range exprs {
		o, err := b.compileSeq(swtch.Cases[i].Path, []sbuf.Puller{switcher.AddCase(e)})
		if err != nil {
			return nil, err
		}
		ops = append(ops, o...)
	}
	return ops, nil
}

// compile compiles a DAG into a graph of runtime operators, and returns
// the leaves.
func (b *Builder) compile(o dag.Op, parents []sbuf.Puller) ([]sbuf.Puller, error) {
	switch o := o.(type) {
	case *dag.ForkOp:
		return b.compileFork(o, parents)
	case *dag.ScatterOp:
		return b.compileScatter(o, parents)
	case *dag.MirrorOp:
		return b.compileMirror(o, parents)
	case *dag.SwitchOp:
		if o.Expr != nil {
			return b.compileExprSwitch(o, parents)
		}
		return b.compileSwitch(o, parents)
	case *dag.HashJoinOp, *dag.JoinOp:
		if len(parents) != 2 {
			return nil, ErrJoinParents
		}
		vectorParents := []vector.Puller{
			vam.NewDematerializer(parents[0]),
			vam.NewDematerializer(parents[1]),
		}
		vectorPuller, err := b.compileVam(o, vectorParents)
		if err != nil {
			return nil, err
		}
		return []sbuf.Puller{vam.NewMaterializer(vectorPuller[0])}, nil
	case *dag.MergeOp:
		exprs, err := b.compileSortExprs(o.Exprs)
		if err != nil {
			return nil, err
		}
		cmp := expr.NewComparator(exprs...).WithMissingAsNull()
		return []sbuf.Puller{merge.New(b.rctx, parents, cmp.Compare)}, nil
	case *dag.CombineOp:
		return []sbuf.Puller{combine.New(b.rctx, parents)}, nil
	default:
		p, err := b.compileLeaf(o, b.combine(parents))
		if err != nil {
			return nil, err
		}
		return []sbuf.Puller{p}, nil
	}
}

func (b *Builder) compilePoolScan(scan *dag.PoolScan) (sbuf.Puller, error) {
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
func NewPushdown(b *Builder, e dag.Expr) sbuf.Pushdown {
	return b.newPushdown(e, nil)
}
func (b *Builder) newPushdown(e dag.Expr, projection []field.Path) sbuf.Pushdown {
	if e == nil && projection == nil {
		return nil
	}
	return &pushdown{
		dataFilter: e,
		builder:    b,
		projection: field.NewProjection(projection),
	}
}

func (b *Builder) newMetaPushdown(e dag.Expr, projection, metaProjection []field.Path, unordered bool) *pushdown {
	return &pushdown{
		metaFilter:     e,
		builder:        b,
		projection:     field.NewProjection(projection),
		metaProjection: field.NewProjection(metaProjection),
		unordred:       unordered,
	}
}

func (b *Builder) lookupPool(id ksuid.KSUID) (*db.Pool, error) {
	if b.env == nil || b.env.DB() == nil {
		return nil, errors.New("internal error: database operation requires database operating context")
	}
	// This is fast because of the pool cache in the database.
	return b.env.DB().OpenPool(b.rctx.Context, id)
}

func (b *Builder) combine(pullers []sbuf.Puller) sbuf.Puller {
	switch len(pullers) {
	case 0:
		return nil
	case 1:
		return pullers[0]
	default:
		return combine.New(b.rctx, pullers)
	}
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
	return e.Eval(b.sctx().Missing()), nil
}

func compileExpr(in dag.Expr) (expr.Evaluator, error) {
	b := NewBuilder(runtime.NewContext(context.Background(), super.NewContext()), nil)
	return b.compileExpr(in)
}

func EvalAtCompileTime(sctx *super.Context, main *dag.MainExpr) (val super.Value, err error) {
	// We pass in a nil adaptor, which causes a panic for anything adaptor
	// related, which is not currently allowed in an expression sub-query.
	b := NewBuilder(runtime.NewContext(context.Background(), sctx), nil)
	for _, f := range main.Funcs {
		b.funcs[f.Tag] = f
	}
	return b.evalAtCompileTime(main.Expr)
}

func isEntry(seq dag.Seq) bool {
	if len(seq) == 0 {
		return false
	}
	switch op := seq[0].(type) {
	case *dag.ListerScan, *dag.DefaultScan, *dag.FileScan, *dag.HTTPScan, *dag.PoolScan, *dag.DBMetaScan, *dag.PoolMetaScan, *dag.CommitMetaScan, *dag.NullScan:
		return true
	case *dag.ForkOp:
		return len(op.Paths) > 0 && !slices.ContainsFunc(op.Paths, func(seq dag.Seq) bool {
			return !isEntry(seq)
		})
	}
	return false
}
