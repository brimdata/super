package shape

import (
	"sync"

	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/sbuf"
)

var MemMaxBytes = 128 * 1024 * 1024

type Op struct {
	rctx   *runtime.Context
	parent sbuf.Puller

	shaper   *Shaper
	once     sync.Once
	resultCh chan op.Result
}

func New(rctx *runtime.Context, parent sbuf.Puller) (*Op, error) {
	return &Op{
		rctx:     rctx,
		parent:   parent,
		shaper:   NewShaper(rctx.Sctx, MemMaxBytes),
		resultCh: make(chan op.Result),
	}, nil
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
	//XXX see issue #3438
	if done {
		panic("shape done not supported")
	}
	o.once.Do(func() { go o.run() })
	if r, ok := <-o.resultCh; ok {
		return r.Batch, r.Err
	}
	return nil, o.rctx.Err()
}

func (o *Op) run() {
	err := o.pullInput()
	if err == nil {
		err = o.pushOutput()
	}
	o.shutdown(err)
}

func (o *Op) pullInput() error {
	for {
		if err := o.rctx.Err(); err != nil {
			return err
		}
		batch, err := o.parent.Pull(false)
		if err != nil || batch == nil {
			return err
		}
		//XXX see issue #3427.
		if err := sbuf.WriteBatch(o.shaper, batch); err != nil {
			return err
		}
		batch.Unref()
	}
}

func (o *Op) pushOutput() error {
	puller := sbuf.NewPuller(o.shaper)
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

func (o *Op) sendResult(b sbuf.Batch, err error) {
	select {
	case o.resultCh <- op.Result{Batch: b, Err: err}:
	case <-o.rctx.Done():
	}
}

func (o *Op) shutdown(err error) {
	if err2 := o.shaper.Close(); err == nil {
		err = err2
	}
	o.sendResult(nil, err)
	close(o.resultCh)
}
