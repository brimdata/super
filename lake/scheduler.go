package lake

import (
	"context"
	"sync"

	"github.com/brimdata/zed/expr/extent"
	"github.com/brimdata/zed/lake/commit"
	"github.com/brimdata/zed/lake/segment"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zson"
)

type Scheduler struct {
	ctx    context.Context
	zctx   *zson.Context
	pool   *Pool
	snap   *commit.Snapshot
	span   extent.Span
	filter zbuf.Filter
	once   sync.Once
	ch     chan Partition
	done   chan error
	stats  zbuf.ScannerStats
}

func NewSortedScheduler(ctx context.Context, zctx *zson.Context, pool *Pool, snap *commit.Snapshot, span extent.Span, filter zbuf.Filter) *Scheduler {
	return &Scheduler{
		ctx:    ctx,
		zctx:   zctx,
		pool:   pool,
		snap:   snap,
		span:   span,
		filter: filter,
		ch:     make(chan Partition),
		done:   make(chan error),
	}
}

func (s *Scheduler) Stats() zbuf.ScannerStats {
	return s.stats.Copy()
}

func (s *Scheduler) AddStats(stats zbuf.ScannerStats) {
	s.stats.Add(stats)
}

func (s *Scheduler) PullScanTask() (zbuf.PullerCloser, error) {
	s.once.Do(func() {
		go s.run()
	})
	select {
	case p := <-s.ch:
		if p.Segments == nil {
			return nil, <-s.done
		}
		return s.newSortedScanner(p)
	case <-s.ctx.Done():
		return nil, <-s.done
	}
}

func (s *Scheduler) run() {
	if err := ScanPartitions(s.ctx, s.snap, s.span, s.pool.Layout.Order, s.ch); err != nil {
		s.done <- err
	}
	close(s.ch)
	close(s.done)
}

// PullScanWork returns the next span in the schedule.  This is useful for a
// worker proc that pulls spans from teh scheduler, sends them to a remote
// worker, and streams the results into the runtime DAG.
func (s *Scheduler) PullScanWork() (Partition, error) {
	s.once.Do(func() {
		go s.run()
	})
	select {
	case p := <-s.ch:
		return p, nil
	case <-s.ctx.Done():
		return Partition{}, <-s.done
	}
}

func (s *Scheduler) newSortedScanner(p Partition) (zbuf.PullerCloser, error) {
	return newSortedScanner(s.ctx, s.pool, s.zctx, s.filter, p, s)
}

func ScanSpan(ctx context.Context, snap *commit.Snapshot, span extent.Span, o order.Which, ch chan<- segment.Reference) error {
	for _, seg := range snap.Select(span, o) {
		if span == nil || span.Overlaps(seg.First, seg.Last) {
			select {
			case ch <- *seg:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}

func ScanSpanInOrder(ctx context.Context, snap *commit.Snapshot, span extent.Span, o order.Which, ch chan<- segment.Reference) error {
	segments := snap.Select(span, o)
	sortSegments(o, segments)
	for _, seg := range segments {
		select {
		case ch <- *seg:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// ScanPartitions partitions all segments in snap overlapping
// span into non-overlapping partitions, sorts them by pool key and order,
// and sends them to ch.
func ScanPartitions(ctx context.Context, snap *commit.Snapshot, span extent.Span, o order.Which, ch chan<- Partition) error {
	segments := snap.Select(span, o)
	for _, p := range PartitionSegments(segments, o) {
		if span != nil {
			p.Span.Crop(span)
		}
		select {
		case ch <- p:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
