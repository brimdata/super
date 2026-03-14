package op

import (
	"context"
	"sync"

	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

type Debug struct {
	parent vio.Puller
	ctx    context.Context
	expr   expr.Evaluator
	filter expr.Evaluator
	ch     chan vector.Any
	eos    <-chan struct{}
	once   sync.Once
}

func NewDebug(ctx context.Context, expr expr.Evaluator, filter expr.Evaluator, chans *DebugChans, parent vio.Puller) *Debug {
	return &Debug{
		parent: parent,
		ctx:    ctx,
		expr:   expr,
		filter: filter,
		ch:     chans.Next(),
		eos:    chans.EOS,
	}
}

func (d *Debug) Pull(done bool) (vector.Any, error) {
	local := make(chan vector.Any)
	d.once.Do(func() {
		go func() {
			for {
				select {
				case vec := <-local:
					d.ch <- vec
				case <-d.eos:
					// send eos ack
					d.ch <- nil
					return
				case <-d.ctx.Done():
					return
				}
			}
		}()
	})
	val, err := d.parent.Pull(done)
	if val == nil {
		return nil, err
	}
	filtered := val
	if d.filter != nil {
		filtered, _ = applyMask(val, d.filter.Eval(filtered))
	}
	if e := d.expr.Eval(filtered); e.Len() != 0 {
		select {
		case local <- e:
		case <-d.ctx.Done():
			return nil, d.ctx.Err()
		}
	}
	return val, err
}
