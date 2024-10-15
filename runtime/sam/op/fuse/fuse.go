package fuse

import (
	"sync"

	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/zbuf"
)

var MemMaxBytes = 128 * 1024 * 1024

type Op struct {
	rctx   *runtime.Context
	parent zbuf.Puller

	fuser    *Fuser
	once     sync.Once
	resultCh chan op.Result
}

func New(rctx *runtime.Context, parent zbuf.Puller) (*Op, error) {
	return &Op{
		rctx:     rctx,
		parent:   parent,
		fuser:    NewFuser(rctx.Zctx, MemMaxBytes),
		resultCh: make(chan op.Result),
	}, nil
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	// XXX ignoring the done indicator.  See issue #3436.
	o.once.Do(func() { go o.run() })
	if r, ok := <-o.resultCh; ok {
		return r.Batch, r.Err
	}
	return nil, o.rctx.Err()
}

func (o *Op) run() {
	if err := o.pullInput(); err != nil {
		o.shutdown(err)
		return
	}
	o.shutdown(o.pushOutput())
}

func (o *Op) pullInput() error {
	for {
		if err := o.rctx.Err(); err != nil {
			return err
		}
		batch, err := o.parent.Pull(false)
		if err != nil {
			return err
		}
		if batch == nil {
			return nil
		}
		if err := zbuf.WriteBatch(o.fuser, batch); err != nil {
			return err
		}
		batch.Unref()
	}
}

func (o *Op) pushOutput() error {
	puller := zbuf.NewPuller(o.fuser)
	for {
		if err := o.rctx.Err(); err != nil {
			return err
		}
		batch, err := puller.Pull(false)
		if err != nil || batch == nil {
			return err
		}
		o.sendResult(batch, nil)
	}
}

func (o *Op) sendResult(b zbuf.Batch, err error) {
	select {
	case o.resultCh <- op.Result{Batch: b, Err: err}:
	case <-o.rctx.Done():
	}
}

func (o *Op) shutdown(err error) {
	if err2 := o.fuser.Close(); err == nil {
		err = err2
	}
	o.sendResult(nil, err)
	close(o.resultCh)
}
