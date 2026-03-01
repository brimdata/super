package debug

import (
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sbuf"
)

type Op struct {
	parent sbuf.Puller
	rctx   *runtime.Context
	expr   expr.Evaluator
	ch     chan sbuf.Batch
}

func New(rctx *runtime.Context, expr expr.Evaluator, parent sbuf.Puller) (*Op, <-chan sbuf.Batch) {
	ch := make(chan sbuf.Batch)
	return &Op{
		parent: parent,
		rctx:   rctx,
		expr:   expr,
		ch:     ch,
	}, ch
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
	batch, err := o.parent.Pull(done)
	if batch == nil || err != nil {
		return batch, err
	}
	if debug := o.evalBatch(batch); len(debug.Values()) != 0 {
		select {
		case o.ch <- debug:
		case <-o.rctx.Done():
			return nil, o.rctx.Err()
		}
	}
	return batch, err
}

func (o *Op) evalBatch(in sbuf.Batch) sbuf.Batch {
	var out sbuf.Array
	for _, x := range in.Values() {
		out.Append(o.expr.Eval(x))
	}
	return &out
}
