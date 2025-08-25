package mirror

import (
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/sbuf"
)

type Op struct {
	parent   sbuf.Puller
	rctx     *runtime.Context
	mirrored *mirrored
}

func New(rctx *runtime.Context, parent sbuf.Puller) *Op {
	m := &Op{
		parent: parent,
		rctx:   rctx,
	}
	s := &mirrored{
		op:       m,
		doneCh:   make(chan struct{}),
		resultCh: make(chan op.Result),
	}
	m.mirrored = s
	return m
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
	batch, err := o.parent.Pull(done)
	if batch == nil || err != nil {
		o.sendEOS(err)
		return batch, err
	}
	if !o.mirrored.blocked {
		batch.Ref()
		select {
		case o.mirrored.resultCh <- op.Result{Batch: batch}:
		case <-o.mirrored.doneCh:
			batch.Unref()
			o.mirrored.blocked = true
		case <-o.rctx.Done():
			return nil, o.rctx.Err()
		}
	}
	return batch, err
}

func (o *Op) sendEOS(err error) {
	if !o.mirrored.blocked {
		select {
		case o.mirrored.resultCh <- op.Result{Err: err}:
			o.mirrored.blocked = true
		case <-o.mirrored.doneCh:
			o.mirrored.blocked = true
		case <-o.rctx.Done():
		}
	}
	o.mirrored.blocked = false
}

func (o *Op) Mirrored() sbuf.Puller {
	return o.mirrored
}

type mirrored struct {
	op       *Op
	resultCh chan op.Result
	doneCh   chan struct{}
	// blocked is managed by Op exclusively.
	blocked bool
}

func (s *mirrored) Pull(done bool) (sbuf.Batch, error) {
	if done {
		select {
		case s.doneCh <- struct{}{}:
			return nil, nil
		case <-s.op.rctx.Done():
			return nil, s.op.rctx.Err()
		}
	}
	select {
	case result := <-s.resultCh:
		return result.Batch, result.Err
	case <-s.op.rctx.Done():
		return nil, s.op.rctx.Err()
	}
}
