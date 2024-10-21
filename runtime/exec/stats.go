package exec

import (
	"context"

	"github.com/brimdata/super"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lake/commits"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/runtime/sam/expr/extent"
)

// XXX for backward compat keep this for now, and return branchstats for pool/main
type PoolStats struct {
	Size int64 `zed:"size"`
	// XXX (nibs) - This shouldn't be a span because keys don't have to be time.
	Span *nano.Span `zed:"span"`
}

func GetPoolStats(ctx context.Context, p *lake.Pool, snap commits.View) (info PoolStats, err error) {
	// XXX this doesn't scale... it should be stored in the snapshot and is
	// not easy to compute in the face of deletes...
	var poolSpan *extent.Generic
	for _, object := range snap.Select(nil, p.SortKeys.Primary().Order) {
		info.Size += object.Size
		if poolSpan == nil {
			poolSpan = extent.NewGenericFromOrder(object.Min, object.Max, order.Asc)
		} else {
			poolSpan.Extend(object.Min)
			poolSpan.Extend(object.Max)
		}
	}
	//XXX need to change API to take return key range
	if poolSpan != nil {
		min := poolSpan.First()
		if min.Type() == super.TypeTime {
			firstTs := super.DecodeTime(min.Bytes())
			lastTs := super.DecodeTime(poolSpan.Last().Bytes())
			if lastTs < firstTs {
				firstTs, lastTs = lastTs, firstTs
			}
			span := nano.NewSpanTs(firstTs, lastTs+1)
			info.Span = &span
		}
	}
	return info, err
}

type BranchStats struct {
	Size int64 `zed:"size"`
	// XXX (nibs) - This shouldn't be a span because keys don't have to be time.
	Span *nano.Span `zed:"span"`
}

func GetBranchStats(ctx context.Context, b *lake.Branch, snap commits.View) (info BranchStats, err error) {
	// XXX this doesn't scale... it should be stored in the snapshot and is
	// not easy to compute in the face of deletes...
	var poolSpan *extent.Generic
	for _, object := range snap.Select(nil, b.Pool().SortKeys.Primary().Order) {
		info.Size += object.Size
		if poolSpan == nil {
			poolSpan = extent.NewGenericFromOrder(object.Min, object.Max, order.Asc)
		} else {
			poolSpan.Extend(object.Min)
			poolSpan.Extend(object.Max)
		}
	}
	//XXX need to change API to take return key range
	if poolSpan != nil {
		min := poolSpan.First()
		if min.Type() == super.TypeTime {
			firstTs := super.DecodeTime(min.Bytes())
			lastTs := super.DecodeTime(poolSpan.Last().Bytes())
			if lastTs < firstTs {
				firstTs, lastTs = lastTs, firstTs
			}
			span := nano.NewSpanTs(firstTs, lastTs+1)
			info.Span = &span
		}
	}
	return info, err
}
