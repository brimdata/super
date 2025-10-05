package optimizer

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/optimizer/demand"
	"github.com/brimdata/super/db"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime/exec"
	"github.com/segmentio/ksuid"
)

type Optimizer struct {
	ctx  context.Context
	env  *exec.Environment
	db   *db.Root
	nent int
}

func New(ctx context.Context, env *exec.Environment) *Optimizer {
	var root *db.Root
	if env != nil {
		root = env.DB()
	}
	return &Optimizer{
		ctx: ctx,
		env: env,
		db:  root,
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
			if f1, ok := seq[i].(*dag.FilterOp); ok {
				if f2, ok := seq[i+1].(*dag.FilterOp); ok {
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
			if _, ok := seq[i].(*dag.PassOp); ok {
				seq.Delete(i, i+1)
				i--
				continue
			}
		}
		if len(seq) == 0 {
			seq = dag.Seq{dag.Pass}
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
		case *dag.UnnestOp:
			if over && op.Body != nil {
				op.Body = walk(op.Body, over, post)
			}
		case *dag.ForkOp:
			for k := range op.Paths {
				op.Paths[k] = walk(op.Paths[k], over, post)
			}
		case *dag.ScatterOp:
			for k := range op.Paths {
				op.Paths[k] = walk(op.Paths[k], over, post)
			}
		case *dag.MirrorOp:
			op.Main = walk(op.Main, over, post)
			op.Mirror = walk(op.Mirror, over, post)
		}
	}
	return post(seq)
}

func walkEntries(seq dag.Seq, post func(dag.Seq) (dag.Seq, error)) (dag.Seq, error) {
	for _, op := range seq {
		switch op := op.(type) {
		case *dag.ForkOp:
			for k := range op.Paths {
				seq, err := walkEntries(op.Paths[k], post)
				if err != nil {
					return nil, err
				}
				op.Paths[k] = seq
			}
		case *dag.ScatterOp:
			for k := range op.Paths {
				seq, err := walkEntries(op.Paths[k], post)
				if err != nil {
					return nil, err
				}
				op.Paths[k] = seq
			}
		case *dag.MirrorOp:
			var err error
			if op.Main, err = walkEntries(op.Main, post); err != nil {
				return nil, err
			}
			if op.Mirror, err = walkEntries(op.Mirror, post); err != nil {
				return nil, err
			}
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
func (o *Optimizer) Optimize(main *dag.Main) error {
	seq := main.Body
	replaceJoinWithHashJoin(seq)
	seq = liftFilterOps(seq)
	seq = mergeFilters(seq)
	seq = mergeValuesOps(seq)
	inlineRecordExprSpreads(seq)
	seq = joinFilterPullup(seq)
	seq = removePassOps(seq)
	seq = replaceSortAndHeadOrTailWithTop(seq)
	o.optimizeParallels(seq)
	seq = mergeFilters(seq)
	seq, err := o.optimizeSourcePaths(seq)
	if err != nil {
		return err
	}
	seq = removePassOps(seq)
	DemandForSeq(seq, demand.All())
	setPushdownUnordered(seq, false)
	main.Body = seq
	return nil
}

func (o *Optimizer) OptimizeDeleter(main *dag.Main, replicas int) error {
	seq := main.Body
	if len(seq) != 3 {
		return errors.New("internal error: bad deleter structure")
	}
	scan, ok := seq[0].(*dag.DeleteScan)
	if !ok {
		return errors.New("internal error: bad deleter structure")
	}
	filter, ok := seq[1].(*dag.FilterOp)
	if !ok {
		return errors.New("internal error: bad deleter structure")
	}
	output, ok := seq[2].(*dag.OutputOp)
	if !ok {
		return errors.New("internal error: bad deleter structure")
	}
	lister := &dag.ListerScan{
		Kind:   "ListerScan",
		Pool:   scan.ID,
		Commit: scan.Commit,
	}
	sortKeys, err := o.sortKeysOfSource(lister)
	if err != nil {
		return err
	}
	deleter := &dag.DeleterScan{
		Kind:  "DeleterScan",
		Pool:  scan.ID,
		Where: filter.Expr,
		//XXX KeyPruner?
	}
	lister.KeyPruner = maybeNewRangePruner(filter.Expr, sortKeys)
	scatter := &dag.ScatterOp{Kind: "ScatterOp"}
	for range replicas {
		scatter.Paths = append(scatter.Paths, dag.CopySeq(dag.Seq{deleter}))
	}
	var merge dag.Op
	if sortKeys.IsNil() {
		merge = &dag.CombineOp{Kind: "CombineOp"}
	} else {
		sortKey := sortKeys.Primary()
		merge = &dag.MergeOp{
			Kind: "MergeOp",
			Exprs: []dag.SortExpr{{
				Key:   dag.NewThis(sortKey.Key),
				Order: sortKey.Order,
				Nulls: sortKey.Order.NullsMax(true),
			}},
		}
	}
	main.Body = dag.Seq{lister, scatter, merge, output}
	return nil
}

func (o *Optimizer) optimizeSourcePaths(seq dag.Seq) (dag.Seq, error) {
	return walkEntries(seq, func(seq dag.Seq) (dag.Seq, error) {
		if len(seq) == 0 {
			return nil, errors.New("internal error: optimizer encountered empty sequential operator")
		}
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
			o.nent++
			// Here we transform a PoolScan into a Lister followed by one or more chains
			// of slicers and sequence scanners.  We'll eventually choose other configurations
			// here based on metadata and availability of CSUP.
			lister := &dag.ListerScan{
				Kind:   "ListerScan",
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
				seq = append(seq, &dag.SlicerOp{Kind: "SlicerOp"})
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
			o.nent++
			if o.env.UseVAM() {
				// Here, we install the filter without a projection.
				// The demand pass comes subsequently and will add
				// the projection.
				op.Pushdown.MetaFilter = newMetaFilter(filter)
				// Vector file readers don't support DataFilter pushdown yet so no need
				// to install the filter here.  But we will eventually and this is
				// where it should be set.
				return seq, nil
			}
			if filter != nil {
				// Filter without projection.  Projection added later.
				op.Pushdown.DataFilter = &dag.ScanFilter{Expr: filter}
			}
			seq = append(dag.Seq{op}, chain...)
		case *dag.CommitMetaScan:
			o.nent++
			if op.Tap {
				sortKeys, err := o.sortKeysOfSource(op)
				if err != nil {
					return nil, err
				}
				// Check to see if we can add a range pruner when the pool key is used
				// in a normal filtering operation.
				op.KeyPruner = maybeNewRangePruner(filter, sortKeys)
				// Delete the downstream operators when we are tapping the object list.
				o, ok := seq[len(seq)-1].(*dag.OutputOp)
				if !ok {
					o = &dag.OutputOp{Kind: "OutputOp", Name: "main"}
				}
				seq = dag.Seq{op, o}
			}
		case *dag.DefaultScan:
			o.nent++
			op.Filter = filter
			seq = append(dag.Seq{op}, chain...)
		}
		return seq, nil
	})
}

func (o *Optimizer) SortKeys(seq dag.Seq) ([]order.SortKeys, error) {
	return o.propagateSortKey(dag.CopySeq(seq), []order.SortKeys{nil})
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
	switch op.(type) {
	case *dag.HashJoinOp, *dag.JoinOp:
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
	case *dag.AggregateOp:
		if parent.IsNil() {
			return []order.SortKeys{nil}, nil
		}
		//XXX handle only primary sortKey for now
		sortKey := parent.Primary()
		for _, k := range op.Keys {
			if groupingKey := fieldOf(k.LHS); groupingKey.Equal(sortKey.Key) {
				rhsExpr := k.RHS
				rhs := fieldOf(rhsExpr)
				if rhs.Equal(sortKey.Key) || orderPreservingCall(rhsExpr, groupingKey) {
					op.InputSortDir = int(sortKey.Order.Direction())
					// Currently, the aggregate operator will sort its
					// output according to the primary key, but we
					// should relax this and do an analysis here as
					// to whether the sort is necessary for the
					// downstream consumer.
					return []order.SortKeys{parent}, nil
				}
			}
		}
		// We'll leave this as unknown for now in spite of the aggregate
		// and not try to optimize downstream of the first aggregate
		// unless there is an excplicit sort encountered.
		return []order.SortKeys{nil}, nil
	case *dag.ForkOp:
		var keys []order.SortKeys
		for _, seq := range op.Paths {
			out, err := o.propagateSortKey(seq, []order.SortKeys{parent})
			if err != nil {
				return nil, err
			}
			keys = append(keys, out...)
		}
		return keys, nil
	case *dag.ScatterOp:
		var keys []order.SortKeys
		for _, seq := range op.Paths {
			out, err := o.propagateSortKey(seq, []order.SortKeys{parent})
			if err != nil {
				return nil, err
			}
			keys = append(keys, out...)
		}
		return keys, nil
	case *dag.MirrorOp:
		var keys []order.SortKeys
		for _, seq := range []dag.Seq{op.Main, op.Mirror} {
			out, err := o.propagateSortKey(seq, []order.SortKeys{parent})
			if err != nil {
				return nil, err
			}
			keys = append(keys, out...)
		}
		return keys, nil
	case *dag.MergeOp:
		var sortKeys order.SortKeys
		sortExpr := op.Exprs[0]
		if this, ok := sortExpr.Key.(*dag.ThisExpr); ok {
			sortKeys = append(sortKeys, order.NewSortKey(sortExpr.Order, this.Path))
		}
		if !sortKeys.Equal(parent) {
			sortKeys = nil
		}
		return []order.SortKeys{sortKeys}, nil
	case *dag.PoolScan, *dag.ListerScan, *dag.SeqScan, *dag.DefaultScan:
		out, err := o.sortKeysOfSource(op)
		return []order.SortKeys{out}, err
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
	case *dag.ListerScan:
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

func (o *Optimizer) lookupPool(id ksuid.KSUID) (*db.Pool, error) {
	if o.db == nil {
		return nil, errors.New("internal error: database operation requires database operating context")
	}
	// This is fast because of the pool cache in the database.
	return o.db.OpenPool(o.ctx, id)
}

// matchFilter attempts to find a filter from the front seq
// and returns the filter's expression (and the modified seq) so
// we can lift the filter predicate into the scanner.
func matchFilter(seq dag.Seq) (dag.Expr, dag.Seq) {
	if len(seq) == 0 {
		return nil, seq
	}
	filter, ok := seq[0].(*dag.FilterOp)
	if !ok {
		return nil, seq
	}
	return filter.Expr, seq[1:]
}

// inlineRecordExprSpreads transforms "{...{a}}" to "{a}".
func inlineRecordExprSpreads(v any) {
	walkT(reflect.ValueOf(v), func(r *dag.RecordExpr) *dag.RecordExpr {
		for i := range r.Elems {
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
		// dedupe elems from spreads
		m := map[string]struct{}{}
		for i := len(r.Elems) - 1; i >= 0; i-- {
			if f, ok := r.Elems[i].(*dag.Field); ok {
				if _, ok := m[f.Name]; ok {
					r.Elems = slices.Delete(r.Elems, i, i+1)
				}
				m[f.Name] = struct{}{}
			}
		}
		return r
	})
}

func joinFilterPullup(seq dag.Seq) dag.Seq {
	seq = mergeFilters(seq)
	for i := 0; i <= len(seq)-3; i++ {
		fork, isfork := seq[i].(*dag.ForkOp)
		leftAlias, rightAlias, isjoin := isJoin(seq[i+1])
		filter, isfilter := seq[i+2].(*dag.FilterOp)
		if !isfork || !isjoin || !isfilter {
			continue
		}
		if len(fork.Paths) != 2 {
			panic(seq[i])
		}
		var remaining []dag.Expr
		for _, e := range splitPredicate(filter.Expr) {
			if pullup, ok := pullupExpr(leftAlias, e); ok {
				fork.Paths[0] = append(fork.Paths[0], dag.NewFilterOp(pullup))
				continue
			}
			if pullup, ok := pullupExpr(rightAlias, e); ok {
				fork.Paths[1] = append(fork.Paths[1], dag.NewFilterOp(pullup))
				continue
			}
			remaining = append(remaining, e)
		}
		if len(remaining) == 0 {
			// Filter has been fully pulled up and can be removed.
			seq.Delete(i+2, i+3)
		} else {
			out := remaining[0]
			for _, e := range remaining[1:] {
				out = dag.NewBinaryExpr("and", e, out)
			}
			seq[i+2] = dag.NewFilterOp(out)
		}
		fork.Paths[0] = joinFilterPullup(fork.Paths[0])
		fork.Paths[1] = joinFilterPullup(fork.Paths[1])
	}
	return seq
}

func isJoin(op dag.Op) (string, string, bool) {
	switch op := op.(type) {
	case *dag.HashJoinOp:
		return op.LeftAlias, op.RightAlias, true
	case *dag.JoinOp:
		return op.LeftAlias, op.RightAlias, true
	default:
		return "", "", false
	}
}

func splitPredicate(e dag.Expr) []dag.Expr {
	if b, ok := e.(*dag.BinaryExpr); ok && b.Op == "and" {
		return append(splitPredicate(b.LHS), splitPredicate(b.RHS)...)
	}
	return []dag.Expr{e}
}

func pullupExpr(alias string, expr dag.Expr) (dag.Expr, bool) {
	e, ok := expr.(*dag.BinaryExpr)
	if !ok {
		return nil, false
	}
	if e.Op == "and" {
		lhs, lok := pullupExpr(alias, e.LHS)
		rhs, rok := pullupExpr(alias, e.RHS)
		if !lok || !rok {
			return nil, false
		}
		return dag.NewBinaryExpr("and", lhs, rhs), true
	}
	if e.Op == "or" {
		lhs, lok := pullupExpr(alias, e.LHS)
		rhs, rok := pullupExpr(alias, e.RHS)
		if !lok || !rok {
			return nil, false
		}
		return dag.NewBinaryExpr("or", lhs, rhs), true

	}
	var literal *dag.LiteralExpr
	var this *dag.ThisExpr
	for _, e := range []dag.Expr{e.RHS, e.LHS} {
		if l, ok := e.(*dag.LiteralExpr); ok && literal == nil {
			literal = l
			continue
		}
		if t, ok := e.(*dag.ThisExpr); ok && this == nil && len(t.Path) > 1 && t.Path[0] == alias {
			this = t
			continue
		}
		return nil, false
	}
	path := slices.Clone(this.Path[1:])
	return dag.NewBinaryExpr(e.Op, dag.NewThis(path), literal), true
}

func liftFilterOps(seq dag.Seq) dag.Seq {
	walkT(reflect.ValueOf(&seq), func(seq dag.Seq) dag.Seq {
		for i := len(seq) - 2; i >= 0; i-- {
			y, ok := seq[i].(*dag.ValuesOp)
			if !ok || len(y.Exprs) != 1 {
				continue
			}
			re, ok1 := y.Exprs[0].(*dag.RecordExpr)
			f, ok2 := seq[i+1].(*dag.FilterOp)
			if !ok1 || !ok2 || hasThisWithEmptyPath(f) {
				continue
			}
			fields, spread, ok := recordElemsFieldsAndSpread(re.Elems)
			if !ok {
				continue
			}
			f = dag.CopyOp(f).(*dag.FilterOp)
			liftOK := true
			walkT(reflect.ValueOf(f), func(e dag.Expr) dag.Expr {
				if !liftOK {
					return e
				}
				this, ok := e.(*dag.ThisExpr)
				if !ok {
					return e
				}
				e1, ok := fields[this.Path[0]]
				if !ok {
					if spread == nil {
						return &dag.LiteralExpr{Kind: "LiteralExpr", Value: `error("missing")`}
					}
					// Copy spread so f and y don't share dag.Exprs.
					e, liftOK = addPathToExpr(dag.CopyExpr(spread), this.Path)
					return e
				}
				// Copy e1 so f and y don't share dag.Exprs.
				e, liftOK = addPathToExpr(dag.CopyExpr(e1), this.Path[1:])
				return e
			})
			if liftOK {
				seq[i], seq[i+1] = f, y
			}
		}
		return seq
	})
	return seq
}

func mergeValuesOps(seq dag.Seq) dag.Seq {
	return walk(seq, true, func(seq dag.Seq) dag.Seq {
		for i := 0; i+1 < len(seq); i++ {
			v1, ok := seq[i].(*dag.ValuesOp)
			if !ok || len(v1.Exprs) != 1 || hasThisWithEmptyPath(seq[i+1]) {
				continue
			}
			re1, ok := v1.Exprs[0].(*dag.RecordExpr)
			if !ok {
				continue
			}
			v1TopLevelFields, v1TopLevelSpread, ok := recordElemsFieldsAndSpread(re1.Elems)
			if !ok {
				continue
			}
			mergeOK := true
			propagateV1Fields := func(e dag.Expr) dag.Expr {
				if !mergeOK {
					return e
				}
				this, ok := e.(*dag.ThisExpr)
				if !ok {
					return e
				}
				v1Expr, ok := v1TopLevelFields[this.Path[0]]
				if !ok {
					if v1TopLevelSpread == nil {
						return &dag.LiteralExpr{Kind: "LiteralExpr", Value: `error("missing")`}
					}
					e, mergeOK = addPathToExpr(v1TopLevelSpread, this.Path)
					return e
				}
				e, mergeOK = addPathToExpr(v1Expr, this.Path[1:])
				return e
			}
			var mergedOp dag.Op
			switch op := seq[i+1].(type) {
			case *dag.AggregateOp:
				op = dag.CopyOp(op).(*dag.AggregateOp)
				for i := range op.Keys {
					walkT(reflect.ValueOf(&op.Keys[i].RHS), propagateV1Fields)
				}
				for i := range op.Aggs {
					walkT(reflect.ValueOf(&op.Aggs[i].RHS), propagateV1Fields)
				}
				mergedOp = op
			case *dag.ValuesOp:
				op = dag.CopyOp(op).(*dag.ValuesOp)
				walkT(reflect.ValueOf(op.Exprs), propagateV1Fields)
				mergedOp = op
			default:
				continue
			}
			if mergeOK {
				inlineRecordExprSpreads(mergedOp)
				seq[i] = mergedOp
				seq.Delete(i+1, i+2)
				i--
			}
		}
		return seq
	})
}

func hasThisWithEmptyPath(v any) bool {
	var found bool
	walkT(reflect.ValueOf(v), func(this *dag.ThisExpr) *dag.ThisExpr {
		if len(this.Path) < 1 {
			found = true
		}
		return this
	})
	return found
}

// addPathToExpr is roughly equivalent to this:
//
//	func simpleAddPathToExpr((e dag.Expr, path []string) dag.Expr {
//	   for _, elem := range path {
//	       e = &dag.Dot{Kind: "Dot", LHS: e, RHS: elem}
//	   }
//	   return e
//	}
//
// addPathToExpr differs in a few ways:
//   - It returns a dag.This when possible.
//   - It descends to a dag.RecordExpr.Elem when possible.
//   - It returns false when it cannot descend to a dag.RecordExpr.Elem.
func addPathToExpr(e dag.Expr, path []string) (dag.Expr, bool) {
	if len(path) == 0 {
		return e, true
	}
	switch e := e.(type) {
	case *dag.RecordExpr:
		var spread *dag.Spread
		for _, elem := range slices.Backward(e.Elems) {
			switch elem := elem.(type) {
			case *dag.Field:
				if elem.Name != path[0] {
					continue
				}
				if spread != nil {
					// Don't know which will win.
					return e, false
				}
				return addPathToExpr(elem.Value, path[1:])
			case *dag.Spread:
				if spread != nil {
					// Don't know which will win.
					return e, false
				}
				spread = elem
			}
		}
		if spread == nil {
			return e, false
		}
		return addPathToExpr(spread.Expr, path)
	case *dag.ThisExpr:
		return dag.NewThis(slices.Concat(e.Path, path)), true
	}
	for _, elem := range path {
		e = &dag.DotExpr{Kind: "DotExpr", LHS: e, RHS: elem}
	}
	return e, true
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

func replaceSortAndHeadOrTailWithTop(seq dag.Seq) dag.Seq {
	walkT(reflect.ValueOf(&seq), func(seq dag.Seq) dag.Seq {
		for i := 0; i+1 < len(seq); i++ {
			sort, ok := seq[i].(*dag.SortOp)
			if !ok {
				continue
			}
			var limit int
			exprs := sort.Exprs
			reverse := sort.Reverse
			switch op := seq[i+1].(type) {
			case *dag.HeadOp:
				limit = op.Count
			case *dag.TailOp:
				limit = op.Count
				for i, e := range exprs {
					exprs[i].Order = !e.Order
				}
				reverse = !reverse
			default:
				continue
			}
			if limit > 1048576 {
				// Limit memory consumption since top doesn't
				// spill to disk.
				continue
			}
			seq[i] = &dag.TopOp{
				Kind:    "TopOp",
				Limit:   limit,
				Exprs:   exprs,
				Reverse: reverse && len(exprs) == 0,
			}
			seq.Delete(i+1, i+2)
		}
		return seq
	})
	return seq
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

// setPushdownUnordered walks seq, setting dag.Pushdown.Unordered to reflect
// whether the containing scan can be unordered (i.e., need not preserve the
// order of values in the underlying data source).  setPushdownUnordered returns
// whether seq's input can be unordered.
func setPushdownUnordered(seq dag.Seq, unordered bool) bool {
	for i := len(seq) - 1; i >= 0; i-- {
		switch op := seq[i].(type) {
		case *dag.AggregateOp, *dag.CombineOp, *dag.DistinctOp, *dag.HashJoinOp, *dag.JoinOp, *dag.SortOp, *dag.TopOp,
			*dag.DefaultScan, *dag.HTTPScan, *dag.PoolScan,
			*dag.CommitMetaScan, *dag.DBMetaScan, *dag.PoolMetaScan:
			unordered = true
		case *dag.FileScan:
			op.Pushdown.Unordered = unordered
			unordered = true
		case *dag.ForkOp:
			for _, p := range op.Paths {
				setPushdownUnordered(p, true)
			}
			unordered = true
		case *dag.MergeOp:
			unordered = false
		case *dag.MirrorOp:
			unordered = setPushdownUnordered(op.Main, unordered)
		case *dag.ScatterOp:
			for _, p := range op.Paths {
				setPushdownUnordered(p, true)
			}
			unordered = true
		case *dag.SwitchOp:
			for _, c := range op.Cases {
				setPushdownUnordered(c.Path, true)
			}
			unordered = true
		}
	}
	return unordered
}
