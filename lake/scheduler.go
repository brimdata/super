package lake

import (
	"context"
	"sync"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/lake/commits"
	"github.com/brimdata/zed/lake/data"
	"github.com/brimdata/zed/lake/index"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/runtime/expr"
	"github.com/brimdata/zed/runtime/expr/extent"
	"github.com/brimdata/zed/runtime/op"
	"github.com/brimdata/zed/zbuf"
	"golang.org/x/sync/errgroup"
)

type Scheduler struct {
	ctx         context.Context
	zctx        *zed.Context
	pool        *Pool
	snap        commits.View
	filter      zbuf.Filter
	rangeFinder rangeFinder
	once        sync.Once
	ch          chan Partition
	group       *errgroup.Group
	progress    zbuf.Progress
}

var _ op.Scheduler = (*Scheduler)(nil)

func NewSortedScheduler(ctx context.Context, zctx *zed.Context, pool *Pool, snap commits.View, filter zbuf.Filter) (*Scheduler, error) {
	ranger, err := newRangeFinder(pool, snap, filter)
	if err != nil {
		return nil, err
	}
	return &Scheduler{
		ctx:         ctx,
		zctx:        zctx,
		pool:        pool,
		rangeFinder: ranger,
		snap:        snap,
		filter:      filter,
		ch:          make(chan Partition),
	}, nil
}

func (s *Scheduler) Progress() zbuf.Progress {
	return s.progress.Copy()
}

func (s *Scheduler) PullScanTask() (zbuf.Puller, error) {
	s.once.Do(func() {
		s.run()
	})
	select {
	case p := <-s.ch:
		if p.Objects == nil {
			return nil, s.group.Wait()
		}
		return newSortedScanner(s, p)
	case <-s.ctx.Done():
		return nil, s.group.Wait()
	}
}

func (s *Scheduler) run() {
	var ctx context.Context
	s.group, ctx = errgroup.WithContext(s.ctx)
	s.group.Go(func() error {
		defer close(s.ch)
		return ScanPartitions(ctx, s.snap, s.pool.Layout, s.filter, s.ch)
	})
}

// PullScanWork returns the next span in the schedule.  This is useful for a
// worker proc that pulls spans from teh scheduler, sends them to a remote
// worker, and streams the results into the runtime DAG.
func (s *Scheduler) PullScanWork() (Partition, error) {
	s.once.Do(func() {
		s.run()
	})
	select {
	case p := <-s.ch:
		return p, nil
	case <-s.ctx.Done():
		return Partition{}, s.group.Wait()
	}
}

type scannerScheduler struct {
	scanners []zbuf.Scanner
	progress zbuf.Progress
	last     zbuf.Scanner
}

var _ op.Scheduler = (*scannerScheduler)(nil)

func newScannerScheduler(scanners ...zbuf.Scanner) *scannerScheduler {
	return &scannerScheduler{
		scanners: scanners,
	}
}

func (s *scannerScheduler) PullScanTask() (zbuf.Puller, error) {
	if s.last != nil {
		s.progress.Add(s.last.Progress())
		s.last = nil
	}
	if len(s.scanners) > 0 {
		s.last = s.scanners[0]
		s.scanners = s.scanners[1:]
		return s.last, nil
	}
	return nil, nil
}

func (s *scannerScheduler) Progress() zbuf.Progress {
	return s.progress.Copy()
}

func Scan(ctx context.Context, snap commits.View, o order.Which, ch chan<- data.Object) error {
	for _, object := range snap.Select(nil, o) {
		select {
		case ch <- *object:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func ScanInOrder(ctx context.Context, snap commits.View, o order.Which, ch chan<- data.Object) error {
	objects := snap.Select(nil, o)
	sortObjects(o, objects)
	for _, object := range objects {
		select {
		case ch <- *object:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func filterObjects(objects []*data.Object, filter *expr.ObjectFilter, o order.Which) []*data.Object {
	out := objects[:0]
	for _, obj := range objects {
		span := extent.NewGeneric(obj.First, obj.Last, expr.NewValueCompareFn(o == order.Asc))
		if filter == nil || !filter.Eval(span.First(), span.Last()) {
			out = append(out, obj)
		}
	}
	return out
}

// ScanPartitions partitions all the data objects in snap overlapping
// span into non-overlapping partitions, sorts them by pool key and order,
// and sends them to ch.
func ScanPartitions(ctx context.Context, snap commits.View, layout order.Layout, filter zbuf.Filter, ch chan<- Partition) error {
	objects := snap.Select(nil, layout.Order)
	f, err := filter.AsObjectFilter(layout.Order, layout.Primary())
	if err != nil {
		return err
	}
	objects = filterObjects(objects, f, layout.Order)
	for _, p := range PartitionObjects(objects, layout.Order) {
		select {
		case ch <- p:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func ScanIndexes(ctx context.Context, snap commits.View, o order.Which, ch chan<- *index.Object) error {
	for _, idx := range snap.SelectIndexes(nil, o) {
		select {
		case ch <- idx:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
