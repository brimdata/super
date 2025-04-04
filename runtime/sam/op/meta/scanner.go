package meta

import (
	"context"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lake/commits"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/segmentio/ksuid"
)

func NewLakeMetaScanner(ctx context.Context, zctx *super.Context, r *lake.Root, meta string) (zbuf.Scanner, error) {
	var vals []super.Value
	var err error
	switch meta {
	case "pools":
		vals, err = r.BatchifyPools(ctx, zctx, nil)
	case "branches":
		vals, err = r.BatchifyBranches(ctx, zctx, nil)
	default:
		return nil, fmt.Errorf("unknown lake metadata type: %q", meta)
	}
	if err != nil {
		return nil, err
	}
	return zbuf.NewScanner(ctx, zbuf.NewArray(vals), nil)
}

func NewPoolMetaScanner(ctx context.Context, zctx *super.Context, r *lake.Root, poolID ksuid.KSUID, meta string) (zbuf.Scanner, error) {
	p, err := r.OpenPool(ctx, poolID)
	if err != nil {
		return nil, err
	}
	var vals []super.Value
	switch meta {
	case "branches":
		m := sup.NewBSUPMarshalerWithContext(zctx)
		m.Decorate(sup.StylePackage)
		vals, err = p.BatchifyBranches(ctx, zctx, nil, m, nil)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown pool metadata type: %q", meta)
	}
	return zbuf.NewScanner(ctx, zbuf.NewArray(vals), nil)
}

func NewCommitMetaScanner(ctx context.Context, zctx *super.Context, r *lake.Root, poolID, commit ksuid.KSUID, meta string, pruner expr.Evaluator) (zbuf.Puller, error) {
	p, err := r.OpenPool(ctx, poolID)
	if err != nil {
		return nil, err
	}
	switch meta {
	case "objects":
		lister, err := NewSortedLister(ctx, zctx, p, commit, pruner)
		if err != nil {
			return nil, err
		}
		return zbuf.NewScanner(ctx, zbuf.PullerReader(lister), nil)
	case "partitions":
		lister, err := NewSortedLister(ctx, zctx, p, commit, pruner)
		if err != nil {
			return nil, err
		}
		slicer, err := NewSlicer(lister, zctx), nil
		if err != nil {
			return nil, err
		}
		return zbuf.NewScanner(ctx, zbuf.PullerReader(slicer), nil)
	case "log":
		tips, err := p.BatchifyBranchTips(ctx, zctx, nil)
		if err != nil {
			return nil, err
		}
		tipsScanner, err := zbuf.NewScanner(ctx, zbuf.NewArray(tips), nil)
		if err != nil {
			return nil, err
		}
		log := p.OpenCommitLog(ctx, zctx, commit)
		logScanner, err := zbuf.NewScanner(ctx, log, nil)
		if err != nil {
			return nil, err
		}
		return zbuf.MultiScanner(tipsScanner, logScanner), nil
	case "rawlog":
		reader, err := p.OpenCommitLogAsBSUP(ctx, zctx, commit)
		if err != nil {
			return nil, err
		}
		return zbuf.NewScanner(ctx, reader, nil)
	case "vectors":
		snap, err := p.Snapshot(ctx, commit)
		if err != nil {
			return nil, err
		}
		vectors := commits.Vectors(snap)
		reader, err := objectReader(ctx, zctx, vectors, p.SortKeys.Primary().Order)
		if err != nil {
			return nil, err
		}
		return zbuf.NewScanner(ctx, reader, nil)
	default:
		return nil, fmt.Errorf("unknown commit metadata type: %q", meta)
	}
}

func objectReader(ctx context.Context, zctx *super.Context, snap commits.View, order order.Which) (zio.Reader, error) {
	objects := snap.Select(nil, order)
	m := sup.NewBSUPMarshalerWithContext(zctx)
	m.Decorate(sup.StylePackage)
	return readerFunc(func() (*super.Value, error) {
		if len(objects) == 0 {
			return nil, nil
		}
		val, err := m.Marshal(objects[0])
		objects = objects[1:]
		return &val, err
	}), nil
}

type readerFunc func() (*super.Value, error)

func (r readerFunc) Read() (*super.Value, error) { return r() }
