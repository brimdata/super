package op

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/db"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
	"github.com/segmentio/ksuid"
)

type Load struct {
	rctx    *runtime.Context
	root    *db.Root
	parent  vio.Puller
	pool    ksuid.KSUID
	branch  string
	author  string
	message string
	meta    string
	done    bool
}

func NewLoad(rctx *runtime.Context, root *db.Root, parent vio.Puller, pool ksuid.KSUID, branch, author, message, meta string) *Load {
	return &Load{
		rctx:    rctx,
		root:    root,
		parent:  parent,
		pool:    pool,
		branch:  branch,
		author:  author,
		message: message,
		meta:    meta,
	}
}

func (l *Load) Pull(done bool) (vector.Any, error) {
	if l.done {
		l.done = false
		return nil, nil
	}
	if done {
		if _, err := l.parent.Pull(true); err != nil {
			return nil, err
		}
		l.done = false
		return nil, nil
	}
	if len(l.branch) == 0 {
		l.branch = "main"
	}
	l.done = true
	pool, err := l.root.OpenPool(l.rctx.Context, l.pool)
	if err != nil {
		return nil, err
	}
	branch, err := pool.OpenBranchByName(l.rctx.Context, l.branch)
	if err != nil {
		return nil, err
	}
	reader := sbuf.PullerReader(sbuf.NewMaterializer(l.parent))
	commitID, err := branch.Load(l.rctx.Context, l.rctx.Sctx, reader, l.author, l.message, l.meta)
	if err != nil {
		return nil, err
	}
	val := super.NewBytes(commitID[:])
	return sbuf.ValToVec(l.rctx.Sctx, val), nil
}
