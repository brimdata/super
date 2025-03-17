package optimizer

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/optimizer/demand"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime/exec"
	"github.com/segmentio/ksuid"
)

type Optimizer struct {
	ctx  context.Context
	env  *exec.Environment
	lake *lake.Root
	nent int
}

func New(ctx context.Context, env *exec.Environment) *Optimizer {
	var lk *lake.Root
	if env != nil {
		lk = env.Lake()
	}
	return &Optimizer{
		ctx:  ctx,
		env:  env,
		lake: lk,
	}
}

// mergeFilters transforms the DAG by merging adjacent filter operators so that,
// e.g., "where a | where b" becomes "where a and b".
//
// Note: mergeFilters does not descend into dag.OverExpr.Scope, so it cannot
// merge filters in "over" expressions like "sum(over a | where b | where c)".
func mergeFilters(seq dag.Seq) dag.Seq {
	return walk(seq, true, func(seq dag.Seq) dag.Seq {
		// Start at the next to last element and work toward the first.
		for i := len(seq) - 2; i >= 0; i-- {
			if f1, ok := seq[i].(*dag.Filter); ok {
				if f2, ok := seq[i+1].(*dag.Filter); ok {
					// Merge the second filter into the
					// first and then delete the second.
					f1.Expr = dag.NewBinaryExpr("and", f1.Expr, f2.Expr)
					seq.Delete(i+1, i+2)
				}
			}
		}
		return seq
	})
}

func removePassOps(seq dag.Seq) dag.Seq {
	return walk(seq, true, func(seq dag.Seq) dag.Seq {
		for i := 0; i < len(seq); i++ {
			if _, ok := seq[i].(*dag.Pass); ok {
				seq.Delete(i, i+1)
				i--
				continue
			}
		}
		if len(seq) == 0 {
			seq = dag.Seq{dag.PassOp}
		}
		return seq
	})
}

func Walk(seq dag.Seq, post func(dag.Seq) dag.Seq) dag.Seq {
	return walk(seq, true, post)
}

func walk(seq dag.Seq, over bool, post func(dag.Seq) dag.Seq) dag.Seq {
	for _, op := range seq {
		switch op := op.(type) {
		case *dag.Over:
			if over && op.Body != nil {
				op.Body = walk(op.Body, over, post)
			}
		case *dag.Fork:
			for k := range op.Paths {
				op.Paths[k] = walk(op.Paths[k], over, post)
			}
		case *dag.Scatter:
			for k := range op.Paths {
				op.Paths[k] = walk(op.Paths[k], over, post)
			}
		case *dag.Mirror:
			op.Main = walk(op.Main, over, post)
			op.Mirror = walk(op.Mirror, over, post)
		case *dag.Scope:
			op.Body = walk(op.Body, over, post)
		}
	}
	return post(seq)
}

func walkEntries(seq dag.Seq, post func(dag.Seq) (dag.Seq, error)) (dag.Seq, error) {
	for _, op := range seq {
		switch op := op.(type) {
		case *dag.Fork:
			for k := range op.Paths {
				seq, err := walkEntries(op.Paths[k], post)
				if err != nil {
					return nil, err
				}
				op.Paths[k] = seq
			}
		case *dag.Scatter:
			for k := range op.Paths {
				seq, err := walkEntries(op.Paths[k], post)
				if err != nil {
					return nil, err
				}
				op.Paths[k] = seq
			}
		case *dag.Mirror:
			var err error
			if op.Main, err = walkEntries(op.Main, post); err != nil {
				return nil, err
			}
			if op.Mirror, err = walkEntries(op.Mirror, post); err != nil {
				return nil, err
			}
		case *dag.Scope:
			seq, err := walkEntries(op.Body, post)
			if err != nil {
				return nil, err
			}
			op.Body = seq
		}
	}
	return post(seq)
}

// Optimize transforms the DAG by attempting to lift stateless operators
// from the downstream sequence into the trunk of each data source in the From
// operator at the entry point of the DAG.  Once these paths are lifted,
// it also attempts to move any candidate filtering operations into the
// source's pushdown predicate.  This should be called before ParallelizeScan().
// TBD: we need to do pushdown for search/cut to optimize columnar extraction.
func (o *Optimizer) Optimize(seq dag.Seq) (dag.Seq, error) {
	seq = liftFilterOps(seq)
	seq = mergeFilters(seq)
	seq = mergeYieldOps(seq)
	seq = inlineRecordExprSpreads(seq)
	seq = removePassOps(seq)
	o.optimizeParallels(seq)
	seq = mergeFilters(seq)
	seq, err := o.optimizeSourcePaths(seq)
	if err != nil {
		return nil, err
	}
	seq = removePassOps(seq)
	DemandForSeq(seq, demand.All())
	return seq, nil
}

func (o *Optimizer) OptimizeDeleter(seq dag.Seq, replicas int) (dag.Seq, error) {
	if len(seq) != 3 {
		return nil, errors.New("internal error: bad deleter structure")
	}
	scan, ok := seq[0].(*dag.DeleteScan)
	if !ok {
		return nil, errors.New("internal error: bad deleter structure")
	}
	filter, ok := seq[1].(*dag.Filter)
	if !ok {
		return nil, errors.New("internal error: bad deleter structure")
	}
	output, ok := seq[2].(*dag.Output)
	if !ok {
		return nil, errors.New("internal error: bad deleter structure")
	}
	lister := &dag.Lister{
		Kind:   "Lister",
		Pool:   scan.ID,
		Commit: scan.Commit,
	}
	sortKeys, err := o.sortKeysOfSource(lister)
	if err != nil {
		return nil, err
	}
	deleter := &dag.Deleter{
		Kind:  "Deleter",
		Pool:  scan.ID,
		Where: filter.Expr,
		//XXX KeyPruner?
	}
	lister.KeyPruner = maybeNewRangePruner(filter.Expr, sortKeys)
	scatter := &dag.Scatter{Kind: "Scatter"}
	for k := 0; k < replicas; k++ {
		scatter.Paths = append(scatter.Paths, copySeq(dag.Seq{deleter}))
	}
	var merge dag.Op
	if sortKeys.IsNil() {
		merge = &dag.Combine{Kind: "Combine"}
	} else {
		merge = &dag.Merge{
			Kind:  "Merge",
			Expr:  &dag.This{Kind: "This", Path: sortKeys.Primary().Key},
			Order: sortKeys.Primary().Order,
		}
	}
	return dag.Seq{lister, scatter, merge, output}, nil
}

func (o *Optimizer) optimizeSourcePaths(seq dag.Seq) (dag.Seq, error) {
	return walkEntries(seq, func(seq dag.Seq) (dag.Seq, error) {
		if len(seq) == 0 {
			return nil, errors.New("internal error: optimizer encountered empty sequential operator")
		}
		o.nent++
		chain := seq[1:]
		if len(chain) == 0 {
			// Nothing to push down.
			return seq, nil
		}
		o.propagateSortKey(seq, []order.SortKeys{nil})
		// See if we can lift a filtering predicate into the source op.
		// Filter might be nil in which case we just put the chain back
		// on the source op and zero out the source's filter.
		filter, chain := matchFilter(chain)
		switch op := seq[0].(type) {
		case *dag.PoolScan:
			// Here we transform a PoolScan into a Lister followed by one or more chains
			// of slicers and sequence scanners.  We'll eventually choose other configurations
			// here based on metadata and availability of VNG.
			lister := &dag.Lister{
				Kind:   "Lister",
				Pool:   op.ID,
				Commit: op.Commit,
			}
			// Check to see if we can add a range pruner when the pool key is used
			// in a normal filtering operation.
			sortKeys, err := o.sortKeysOfSource(op)
			if err != nil {
				return nil, err
			}
			lister.KeyPruner = maybeNewRangePruner(filter, sortKeys)
			seq = dag.Seq{lister}
			_, _, orderRequired, err := o.concurrentPath(chain, sortKeys)
			if err != nil {
				return nil, err
			}
			if orderRequired {
				seq = append(seq, &dag.Slicer{Kind: "Slicer"})
			}
			seq = append(seq, &dag.SeqScan{
				Kind:      "SeqScan",
				Pool:      op.ID,
				Commit:    op.Commit,
				Filter:    filter,
				KeyPruner: lister.KeyPruner,
			})
			seq = append(seq, chain...)
		case *dag.FileScan:
			if o.env.UseVAM() {
				// Vector file readers don't support filter pushdown yet.
				op.MetadataPruner = newMetadataPruner(filter)
				return seq, nil
			}
			op.Filter = filter
			seq = append(dag.Seq{op}, chain...)
		case *dag.CommitMetaScan:
			if op.Tap {
				sortKeys, err := o.sortKeysOfSource(op)
				if err != nil {
					return nil, err
				}
				// Check to see if we can add a range pruner when the pool key is used
				// in a normal filtering operation.
				op.KeyPruner = maybeNewRangePruner(filter, sortKeys)
				// Delete the downstream operators when we are tapping the object list.
				o, ok := seq[len(seq)-1].(*dag.Output)
				if !ok {
					o = &dag.Output{Kind: "Output", Name: "main"}
				}
				seq = dag.Seq{op, o}
			}
		case *dag.DefaultScan:
			op.Filter = filter
			seq = append(dag.Seq{op}, chain...)
		}
		return seq, nil
	})
}

func (o *Optimizer) SortKeys(seq dag.Seq) ([]order.SortKeys, error) {
	return o.propagateSortKey(copySeq(seq), []order.SortKeys{nil})
}

// propagateSortKey analyzes a Seq and attempts to push the scan order of the data source
// into the first downstream aggregation.  (We could continue the analysis past that
// point but don't bother yet because we do not yet support any optimization
// past the first aggregation.)  For parallel paths, we propagate
// the scan order if its the same at egress of all of the paths.
func (o *Optimizer) propagateSortKey(seq dag.Seq, parents []order.SortKeys) ([]order.SortKeys, error) {
	if len(seq) == 0 {
		return parents, nil
	}
	for _, op := range seq {
		var err error
		parents, err = o.propagateSortKeyOp(op, parents)
		if err != nil {
			return []order.SortKeys{nil}, err
		}
	}
	return parents, nil
}

func (o *Optimizer) propagateSortKeyOp(op dag.Op, parents []order.SortKeys) ([]order.SortKeys, error) {
	if join, ok := op.(*dag.Join); ok {
		if len(parents) != 2 {
			return nil, errors.New("internal error: join does not have two parents")
		}
		if !parents[0].IsNil() && fieldOf(join.LeftKey).Equal(parents[0].Primary().Key) {
			join.LeftDir = parents[0].Primary().Order.Direction()
		}
		if !parents[1].IsNil() && fieldOf(join.RightKey).Equal(parents[1].Primary().Key) {
			join.RightDir = parents[1].Primary().Order.Direction()
		}
		// XXX There is definitely a way to propagate the sort key but there's
		// some complexity here. The propagated sort key should be whatever key
		// remains between the left and right join keys. If both the left and
		// right key are part of the resultant record what should the
		// propagated sort key be? Ideally in this case they both would which
		// would allow for maximum flexibility. For now just return unspecified
		// sort order.
		return []order.SortKeys{nil}, nil
	}
	// If the op is not a join then condense sort order into a single parent,
	// since all the ops only care about the sort order of multiple parents if
	// the SortKey of all parents is unified.
	var parent order.SortKeys
	for k, p := range parents {
		if k == 0 {
			parent = p
		} else if !parent.Equal(p) {
			parent = nil
			break
		}
	}
	switch op := op.(type) {
	case *dag.Summarize:
		if parent.IsNil() {
			return []order.SortKeys{nil}, nil
		}
		//XXX handle only primary sortKey for now
		sortKey := parent.Primary()
		for _, k := range op.Keys {
			if groupByKey := fieldOf(k.LHS); groupByKey.Equal(sortKey.Key) {
				rhsExpr := k.RHS
				rhs := fieldOf(rhsExpr)
				if rhs.Equal(sortKey.Key) || orderPreservingCall(rhsExpr, groupByKey) {
					op.InputSortDir = int(sortKey.Order.Direction())
					// Currently, the groupby operator will sort its
					// output according to the primary key, but we
					// should relax this and do an analysis here as
					// to whether the sort is necessary for the
					// downstream consumer.
					return []order.SortKeys{parent}, nil
				}
			}
		}
		// We'll live this as unknown for now even though the groupby
		// and not try to optimize downstream of the first groupby
		// unless there is an excplicit sort encountered.
		return []order.SortKeys{nil}, nil
	case *dag.Fork:
		var keys []order.SortKeys
		for _, seq := range op.Paths {
			out, err := o.propagateSortKey(seq, []order.SortKeys{parent})
			if err != nil {
				return nil, err
			}
			keys = append(keys, out...)
		}
		return keys, nil
	case *dag.Scatter:
		var keys []order.SortKeys
		for _, seq := range op.Paths {
			out, err := o.propagateSortKey(seq, []order.SortKeys{parent})
			if err != nil {
				return nil, err
			}
			keys = append(keys, out...)
		}
		return keys, nil
	case *dag.Mirror:
		var keys []order.SortKeys
		for _, seq := range []dag.Seq{op.Main, op.Mirror} {
			out, err := o.propagateSortKey(seq, []order.SortKeys{parent})
			if err != nil {
				return nil, err
			}
			keys = append(keys, out...)
		}
		return keys, nil
	case *dag.Merge:
		var sortKeys order.SortKeys
		if this, ok := op.Expr.(*dag.This); ok {
			sortKeys = append(sortKeys, order.NewSortKey(op.Order, this.Path))
		}
		if !sortKeys.Equal(parent) {
			sortKeys = nil
		}
		return []order.SortKeys{sortKeys}, nil
	case *dag.PoolScan, *dag.Lister, *dag.SeqScan, *dag.DefaultScan:
		out, err := o.sortKeysOfSource(op)
		return []order.SortKeys{out}, err
	case *dag.Scope:
		return o.propagateSortKey(op.Body, parents)
	default:
		out, err := o.analyzeSortKeys(op, parent)
		return []order.SortKeys{out}, err
	}
}

func (o *Optimizer) sortKeysOfSource(op dag.Op) (order.SortKeys, error) {
	switch op := op.(type) {
	case *dag.DefaultScan:
		return op.SortKeys, nil
	case *dag.FileScan:
		return nil, nil
	case *dag.HTTPScan:
		return nil, nil
	case *dag.PoolScan:
		return o.sortKey(op.ID)
	case *dag.Lister:
		return o.sortKey(op.Pool)
	case *dag.SeqScan:
		return o.sortKey(op.Pool)
	case *dag.CommitMetaScan:
		if op.Tap && op.Meta == "objects" {
			// For a tap into the object stream, we compile the downstream
			// DAG as if it were a normal query (so the optimizer can prune
			// objects etc.) but we execute it in the end as a meta-query.
			return o.sortKey(op.Pool)
		}
		return nil, nil //XXX is this right?
	default:
		return nil, fmt.Errorf("internal error: unknown source type %T", op)
	}
}

func (o *Optimizer) sortKey(id ksuid.KSUID) (order.SortKeys, error) {
	pool, err := o.lookupPool(id)
	if err != nil {
		return nil, err
	}
	return pool.SortKeys, nil
}

func (o *Optimizer) lookupPool(id ksuid.KSUID) (*lake.Pool, error) {
	if o.lake == nil {
		return nil, errors.New("internal error: lake operation cannot be used in non-lake context")
	}
	// This is fast because of the pool cache in the lake.
	return o.lake.OpenPool(o.ctx, id)
}

// matchFilter attempts to find a filter from the front seq
// and returns the filter's expression (and the modified seq) so
// we can lift the filter predicate into the scanner.
func matchFilter(seq dag.Seq) (dag.Expr, dag.Seq) {
	if len(seq) == 0 {
		return nil, seq
	}
	filter, ok := seq[0].(*dag.Filter)
	if !ok {
		return nil, seq
	}
	return filter.Expr, seq[1:]
}

// inlineRecordExprSpreads transforms "{...{a}}" to "{a}".
func inlineRecordExprSpreads(seq dag.Seq) dag.Seq {
	walkT(reflect.ValueOf(seq), func(r *dag.RecordExpr) *dag.RecordExpr {
		for i := 0; i < len(r.Elems); i++ {
			s, ok := r.Elems[i].(*dag.Spread)
			if !ok {
				continue
			}
			r2, ok := s.Expr.(*dag.RecordExpr)
			if !ok {
				continue
			}
			r.Elems = slices.Concat(r.Elems[:i], r2.Elems, r.Elems[i+1:])
		}
		return r
	})
	return seq
}

func liftFilterOps(seq dag.Seq) dag.Seq {
	walkT(reflect.ValueOf(&seq), func(seq dag.Seq) dag.Seq {
		for i := len(seq) - 2; i >= 0; i-- {
			y, ok := seq[i].(*dag.Yield)
			if !ok || len(y.Exprs) != 1 {
				continue
			}
			re, ok1 := y.Exprs[0].(*dag.RecordExpr)
			f, ok2 := seq[i+1].(*dag.Filter)
			if !ok1 || !ok2 || hasThisWithEmptyPath(f) {
				continue
			}
			fields, spread, ok := recordElemsFieldsAndSpread(re.Elems)
			if !ok {
				continue
			}
			walkT(reflect.ValueOf(f), func(e dag.Expr) dag.Expr {
				this, ok := e.(*dag.This)
				if !ok {
					return e
				}
				e1, ok := fields[this.Path[0]]
				if !ok {
					if spread != nil {
						return addPathToExpr(spread, this.Path)
					}
					return e
				}
				return addPathToExpr(e1, this.Path[1:])
			})
			seq[i], seq[i+1] = seq[i+1], seq[i]
		}
		return seq
	})
	return seq
}

func mergeYieldOps(seq dag.Seq) dag.Seq {
	return walk(seq, true, func(seq dag.Seq) dag.Seq {
		for i := 0; i+1 < len(seq); i++ {
			y1, ok1 := seq[i].(*dag.Yield)
			y2, ok2 := seq[i+1].(*dag.Yield)
			if !ok1 || !ok2 || len(y1.Exprs) != 1 || hasThisWithEmptyPath(y2) {
				continue
			}
			re1, ok := y1.Exprs[0].(*dag.RecordExpr)
			if !ok {
				continue
			}
			y1TopLevelFields, y1TopLevelSpread, ok := recordElemsFieldsAndSpread(re1.Elems)
			if !ok {
				continue
			}
			walkT(reflect.ValueOf(y2), func(e2 dag.Expr) dag.Expr {
				this2, ok := e2.(*dag.This)
				if !ok {
					return e2
				}
				e1, ok := y1TopLevelFields[this2.Path[0]]
				if !ok {
					if y1TopLevelSpread != nil {
						return addPathToExpr(y1TopLevelSpread, this2.Path)
					}
					return e2
				}
				return addPathToExpr(e1, this2.Path[1:])
			})
			seq.Delete(i, i+1)
			i--
		}
		return seq
	})
}

func hasThisWithEmptyPath(v any) bool {
	var found bool
	walkT(reflect.ValueOf(v), func(this *dag.This) *dag.This {
		if len(this.Path) < 1 {
			found = true
		}
		return this
	})
	return found
}

func addPathToExpr(e dag.Expr, path []string) dag.Expr {
	if len(path) == 0 {
		return e
	}
	if this, ok := e.(*dag.This); ok {
		return &dag.This{Kind: "This", Path: slices.Concat(this.Path, path)}
	}
	dot := &dag.Dot{Kind: "Dot", LHS: e, RHS: path[0]}
	for _, s := range path[1:] {
		dot = &dag.Dot{Kind: "Dot", LHS: dot, RHS: s}
	}
	return dot
}

func recordElemsFieldsAndSpread(elems []dag.RecordElem) (map[string]dag.Expr, dag.Expr, bool) {
	fields := map[string]dag.Expr{}
	var spread dag.Expr
	for i, e := range elems {
		switch e := e.(type) {
		case *dag.Field:
			fields[e.Name] = e.Value
		case *dag.Spread:
			if i > 0 {
				return nil, nil, false
			}
			spread = e.Expr
		default:
			panic(e)
		}
	}
	return fields, spread, true
}

func walkT[T any](v reflect.Value, post func(T) T) {
	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		for i := range v.Len() {
			walkT(v.Index(i), post)
		}
	case reflect.Interface, reflect.Pointer:
		walkT(v.Elem(), post)
	case reflect.Struct:
		for i := range v.NumField() {
			walkT(v.Field(i), post)
		}
	}
	if v.CanSet() {
		if t, ok := v.Interface().(T); ok {
			v.Set(reflect.ValueOf(post(t)))
		}
	}
}
