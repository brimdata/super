package exec

import (
	"context"
	"errors"

	"github.com/brimdata/super"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lake/commits"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/op/meta"
	"github.com/brimdata/super/zbuf"
	"github.com/segmentio/ksuid"
)

func Compact(ctx context.Context, lk *lake.Root, pool *lake.Pool, branchName string, objectIDs []ksuid.KSUID, writeVectors bool, author, message, info string) (ksuid.KSUID, error) {
	if len(objectIDs) < 2 {
		return ksuid.Nil, errors.New("compact: two or more source objects required")
	}
	branch, err := pool.OpenBranchByName(ctx, branchName)
	if err != nil {
		return ksuid.Nil, err
	}
	base, err := pool.Snapshot(ctx, branch.Commit)
	if err != nil {
		return ksuid.Nil, err
	}
	compact := commits.NewSnapshot()
	for _, oid := range objectIDs {
		o, err := base.Lookup(oid)
		if err != nil {
			return ksuid.Nil, err
		}
		compact.AddDataObject(o)
	}
	zctx := super.NewContext()
	lister := meta.NewSortedListerFromSnap(ctx, super.NewContext(), pool, compact, nil)
	rctx := runtime.NewContext(ctx, zctx)
	slicer := meta.NewSlicer(lister, zctx)
	puller := meta.NewSequenceScanner(rctx, slicer, pool, nil, nil, nil)
	w := lake.NewSortedWriter(ctx, zctx, pool, writeVectors)
	if err := zbuf.CopyPuller(w, puller); err != nil {
		puller.Pull(true)
		w.Abort()
		return ksuid.Nil, err
	}
	if err := w.Close(); err != nil {
		w.Abort()
		return ksuid.Nil, err
	}
	commit, err := branch.CommitCompact(ctx, compact.SelectAll(), w.Objects(), w.Vectors(), author, message, info)
	if err != nil {
		w.Abort()
		return ksuid.Nil, err
	}
	return commit, nil
}
