package debug

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/vam/op"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/vector"
)

type Op struct {
	parent sbuf.Puller
	rctx   *runtime.Context
	expr   expr.Evaluator
	filter expr.Evaluator
	ch     chan vector.Any
	eos    <-chan struct{}
	once   sync.Once
}

func New(rctx *runtime.Context, expr expr.Evaluator, filter expr.Evaluator, chans *op.DebugChans, parent sbuf.Puller) *Op {
	return &Op{
		parent: parent,
		rctx:   rctx,
		expr:   expr,
		filter: filter,
		ch:     chans.Next(),
		eos:    chans.EOS,
	}
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
	local := make(chan vector.Any)
	o.once.Do(func() {
		go func() {
			for {
				select {
				case vec := <-local:
					o.ch <- vec
				case <-o.eos:
					// send eos ack
					o.ch <- nil
					return
				case <-o.rctx.Done():
					return
				}
			}
		}()
	})
	batch, err := o.parent.Pull(done)
	if batch == nil || err != nil {
		return batch, err
	}
	if e := o.evalBatch(batch); e.Len() != 0 {
		select {
		case local <- e:
		case <-o.rctx.Done():
			return nil, o.rctx.Err()
		}
	}
	return batch, err
}

func (o *Op) evalBatch(in sbuf.Batch) vector.Any {
	builder := vector.NewDynamicValueBuilder()
	for _, x := range in.Values() {
		if o.filter == nil || o.where(x) {
			builder.Write(o.expr.Eval(x))
		}
	}
	return builder.Build(o.rctx.Sctx)
}

func (o *Op) where(val super.Value) bool {
	val = o.filter.Eval(val)
	return val.Type().ID() == super.IDBool && val.Bool()
}
