package meta

import (
	"bytes"
	"context"
	"sort"
	"sync"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/lake"
	"github.com/brimdata/zed/lake/commits"
	"github.com/brimdata/zed/lake/data"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/runtime/expr"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zson"
	"github.com/segmentio/ksuid"
	"golang.org/x/sync/errgroup"
)

// Lister enumerates all the data.Objects in a scan.  A Slicer downstream may
// optionally organize objects into non-overlapping partitions for merge on read.
// The optimizer may decide when partitions are necessary based on the order
// sensitivity of the downstream flowgraph.
type Lister struct {
	ctx       context.Context
	pool      *lake.Pool
	snap      commits.View
	pruner    *pruner
	group     *errgroup.Group
	marshaler *zson.MarshalZNGContext
	mu        sync.Mutex
	objects   []*data.Object
	err       error
}

var _ zbuf.Puller = (*Lister)(nil)

func NewSortedLister(ctx context.Context, zctx *zed.Context, r *lake.Root, pool *lake.Pool, commit ksuid.KSUID, pruner expr.Evaluator) (*Lister, error) {
	snap, err := pool.Snapshot(ctx, commit)
	if err != nil {
		return nil, err
	}
	return NewSortedListerFromSnap(ctx, zctx, r, pool, snap, pruner), nil
}

func NewSortedListerByID(ctx context.Context, zctx *zed.Context, r *lake.Root, poolID, commit ksuid.KSUID, pruner expr.Evaluator) (*Lister, error) {
	pool, err := r.OpenPool(ctx, poolID)
	if err != nil {
		return nil, err
	}
	return NewSortedLister(ctx, zctx, r, pool, commit, pruner)
}

func NewSortedListerFromSnap(ctx context.Context, zctx *zed.Context, r *lake.Root, pool *lake.Pool, snap commits.View, pruner expr.Evaluator) *Lister {
	m := zson.NewZNGMarshalerWithContext(zctx)
	m.Decorate(zson.StylePackage)
	l := &Lister{
		ctx:       ctx,
		pool:      pool,
		snap:      snap,
		group:     &errgroup.Group{},
		marshaler: m,
	}
	if pruner != nil {
		l.pruner = newPruner(pruner)
	}
	return l
}

func (l *Lister) Snapshot() commits.View {
	return l.snap
}

func (l *Lister) Pull(done bool) (zbuf.Batch, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return nil, l.err
	}
	if l.objects == nil {
		l.objects = initObjectScan(l.snap, l.pool.SortKey)
	}
	for len(l.objects) != 0 {
		o := l.objects[0]
		l.objects = l.objects[1:]
		val, err := l.marshaler.Marshal(o)
		if err != nil {
			l.err = err
			return nil, err
		}
		if !l.pruner.prune(val) {
			return zbuf.NewArray([]zed.Value{*val}), nil
		}
	}
	return nil, nil
}

func initObjectScan(snap commits.View, sortKey order.SortKey) []*data.Object {
	objects := snap.Select(nil, sortKey.Order)
	//XXX at some point sorting should be optional.
	sortObjects(objects, sortKey.Order)
	return objects
}

func sortObjects(objects []*data.Object, o order.Which) {
	cmp := expr.NewValueCompareFn(o, true)
	lessFunc := func(a, b *data.Object) bool {
		aFrom, aTo, bFrom, bTo := &a.Min, &a.Max, &b.Min, &b.Max
		if o == order.Desc {
			aFrom, aTo, bFrom, bTo = aTo, aFrom, bTo, bFrom
		}
		if cmp(aFrom, bFrom) < 0 {
			return true
		}
		if !bytes.Equal(aFrom.Bytes(), bFrom.Bytes()) {
			return false
		}
		if bytes.Equal(aTo.Bytes(), bTo.Bytes()) {
			// If the pool keys are equal for both the first and last values
			// in the object, we return false here so that the stable sort preserves
			// the commit order of the objects in the log. XXX we might want to
			// simply sort by commit timestamp for a more robust API that does not
			// presume commit-order in the object snapshot.
			return false
		}
		return cmp(aTo, bTo) < 0
	}
	sort.SliceStable(objects, func(i, j int) bool {
		return lessFunc(objects[i], objects[j])
	})
}
