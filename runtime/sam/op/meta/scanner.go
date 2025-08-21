package meta

import (
	"context"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lake/commits"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sup"
	"github.com/segmentio/ksuid"
)

func NewLakeMetaScanner(ctx context.Context, sctx *super.Context, r *lake.Root, meta string) (sbuf.Scanner, error) {
	var vals []super.Value
	var err error
	switch meta {
	case "pools":
		vals, err = r.BatchifyPools(ctx, sctx, nil)
	case "branches":
		vals, err = r.BatchifyBranches(ctx, sctx, nil)
	default:
		return nil, fmt.Errorf("unknown lake metadata type: %q", meta)
	}
	if err != nil {
		return nil, err
	}
	return sbuf.NewScanner(ctx, sbuf.NewArray(vals), nil)
}

func NewPoolMetaScanner(ctx context.Context, sctx *super.Context, r *lake.Root, poolID ksuid.KSUID, meta string) (sbuf.Scanner, error) {
	p, err := r.OpenPool(ctx, poolID)
	if err != nil {
		return nil, err
	}
	var vals []super.Value
	switch meta {
	case "branches":
		m := sup.NewBSUPMarshalerWithContext(sctx)
		m.Decorate(sup.StylePackage)
		vals, err = p.BatchifyBranches(ctx, sctx, nil, m, nil)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown pool metadata type: %q", meta)
	}
	return sbuf.NewScanner(ctx, sbuf.NewArray(vals), nil)
}

func NewCommitMetaScanner(ctx context.Context, sctx *super.Context, r *lake.Root, poolID, commit ksuid.KSUID, meta string, pruner expr.Evaluator) (sbuf.Puller, error) {
	p, err := r.OpenPool(ctx, poolID)
	if err != nil {
		return nil, err
	}
	switch meta {
	case "objects":
		lister, err := NewSortedLister(ctx, sctx, p, commit, pruner)
		if err != nil {
			return nil, err
		}
		return sbuf.NewScanner(ctx, sbuf.PullerReader(lister), nil)
	case "partitions":
		lister, err := NewSortedLister(ctx, sctx, p, commit, pruner)
		if err != nil {
			return nil, err
		}
		slicer, err := NewSlicer(lister, sctx), nil
		if err != nil {
			return nil, err
		}
		return sbuf.NewScanner(ctx, sbuf.PullerReader(slicer), nil)
	case "log":
		tips, err := p.BatchifyBranchTips(ctx, sctx, nil)
		if err != nil {
			return nil, err
		}
		tipsScanner, err := sbuf.NewScanner(ctx, sbuf.NewArray(tips), nil)
		if err != nil {
			return nil, err
		}
		log := p.OpenCommitLog(ctx, sctx, commit)
		logScanner, err := sbuf.NewScanner(ctx, log, nil)
		if err != nil {
			return nil, err
		}
		return sbuf.MultiScanner(tipsScanner, logScanner), nil
	case "rawlog":
		reader, err := p.OpenCommitLogAsBSUP(ctx, sctx, commit)
		if err != nil {
			return nil, err
		}
		return sbuf.NewScanner(ctx, reader, nil)
	case "vectors":
		snap, err := p.Snapshot(ctx, commit)
		if err != nil {
			return nil, err
		}
		vectors := commits.Vectors(snap)
		reader, err := objectReader(sctx, vectors, p.SortKeys.Primary().Order)
		if err != nil {
			return nil, err
		}
		return sbuf.NewScanner(ctx, reader, nil)
	default:
		return nil, fmt.Errorf("unknown commit metadata type: %q", meta)
	}
}

func objectReader(sctx *super.Context, snap commits.View, order order.Which) (sio.Reader, error) {
	objects := snap.Select(nil, order)
	m := sup.NewBSUPMarshalerWithContext(sctx)
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
