package load

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/zbuf"
	"github.com/segmentio/ksuid"
)

type Op struct {
	rctx    *runtime.Context
	lk      *lake.Root
	parent  zbuf.Puller
	pool    ksuid.KSUID
	branch  string
	author  string
	message string
	meta    string
	done    bool
}

func New(rctx *runtime.Context, lk *lake.Root, parent zbuf.Puller, pool ksuid.KSUID, branch, author, message, meta string) *Op {
	return &Op{
		rctx:    rctx,
		lk:      lk,
		parent:  parent,
		pool:    pool,
		branch:  branch,
		author:  author,
		message: message,
		meta:    meta,
	}
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	if o.done {
		o.done = false
		return nil, nil
	}
	if done {
		b, err := o.parent.Pull(true)
		if err != nil {
			return nil, err
		}
		if b != nil {
			panic("non-nil done batch")
		}
		o.done = false
		return nil, nil
	}
	if len(o.branch) == 0 {
		o.branch = "main"
	}
	o.done = true
	reader := zbuf.PullerReader(o.parent)
	pool, err := o.lk.OpenPool(o.rctx.Context, o.pool)
	if err != nil {
		return nil, err
	}
	branch, err := pool.OpenBranchByName(o.rctx.Context, o.branch)
	if err != nil {
		return nil, err
	}
	commitID, err := branch.Load(o.rctx.Context, o.rctx.Sctx, reader, o.author, o.message, o.meta)
	if err != nil {
		return nil, err
	}
	val := super.NewBytes(commitID[:])
	return zbuf.NewArray([]super.Value{val}), nil
}
