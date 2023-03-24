package fuse

import (
	"sync"

	"github.com/brimdata/zed/runtime/op"
	"github.com/brimdata/zed/zbuf"
)

var MemMaxBytes = 128 * 1024 * 1024

type Op struct {
	octx   *op.Context
	parent zbuf.Puller

	fuser    *Fuser
	once     sync.Once
	resultCh chan op.Result
}

func New(octx *op.Context, parent zbuf.Puller) (*Op, error) {
	return &Op{
		octx:     octx,
		parent:   parent,
		fuser:    NewFuser(octx.Zctx, MemMaxBytes),
		resultCh: make(chan op.Result),
	}, nil
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	// XXX ignoring the done indicator.  See issue #3436.
	o.once.Do(func() { go o.run() })
	if r, ok := <-o.resultCh; ok {
		return r.Batch, r.Err
	}
	return nil, o.octx.Err()
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
		if err := o.octx.Err(); err != nil {
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
		if err := o.octx.Err(); err != nil {
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
	case <-o.octx.Done():
	}
}

func (o *Op) shutdown(err error) {
	if err2 := o.fuser.Close(); err == nil {
		err = err2
	}
	o.sendResult(nil, err)
	close(o.resultCh)
}
