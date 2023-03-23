package shape

import (
	"sync"

	"github.com/brimdata/zed/runtime/op"
	"github.com/brimdata/zed/zbuf"
)

var MemMaxBytes = 128 * 1024 * 1024

type Proc struct {
	octx   *op.Context
	parent zbuf.Puller

	shaper   *Shaper
	once     sync.Once
	resultCh chan op.Result
}

func New(octx *op.Context, parent zbuf.Puller) (*Proc, error) {
	return &Proc{
		octx:     octx,
		parent:   parent,
		shaper:   NewShaper(octx.Zctx, MemMaxBytes),
		resultCh: make(chan op.Result),
	}, nil
}

func (p *Proc) Pull(done bool) (zbuf.Batch, error) {
	//XXX see issue #3438
	if done {
		panic("shape done not supported")
	}
	p.once.Do(func() { go p.run() })
	if r, ok := <-p.resultCh; ok {
		return r.Batch, r.Err
	}
	return nil, p.octx.Err()
}

func (p *Proc) run() {
	err := p.pullInput()
	if err == nil {
		err = p.pushOutput()
	}
	p.shutdown(err)
}

func (p *Proc) pullInput() error {
	for {
		if err := p.octx.Err(); err != nil {
			return err
		}
		batch, err := p.parent.Pull(false)
		if err != nil || batch == nil {
			return err
		}
		//XXX see issue #3427.
		if err := zbuf.WriteBatch(p.shaper, batch); err != nil {
			return err
		}
		batch.Unref()
	}
}

func (p *Proc) pushOutput() error {
	puller := zbuf.NewPuller(p.shaper)
	for {
		if err := p.octx.Err(); err != nil {
			return err
		}
		batch, err := puller.Pull(false)
		if err != nil || batch == nil {
			return err
		}
		p.sendResult(batch, nil)
	}
}

func (p *Proc) sendResult(b zbuf.Batch, err error) {
	select {
	case p.resultCh <- op.Result{Batch: b, Err: err}:
	case <-p.octx.Done():
	}
}

func (p *Proc) shutdown(err error) {
	if err2 := p.shaper.Close(); err == nil {
		err = err2
	}
	p.sendResult(nil, err)
	close(p.resultCh)
}
