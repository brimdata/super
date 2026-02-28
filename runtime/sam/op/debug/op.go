package debug

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sbuf"
)

type Op struct {
	parent  sbuf.Puller
	rctx    *runtime.Context
	expr    expr.Evaluator
	debugCh chan super.Value
}

func New(rctx *runtime.Context, expr expr.Evaluator, parent sbuf.Puller) *Op {
	return &Op{
		parent:  parent,
		rctx:    rctx,
		expr:    expr,
		debugCh: make(chan super.Value),
	}
}

func (o *Op) Channel() chan super.Value {
	return o.debugCh
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
	batch, err := o.parent.Pull(done)
	if batch == nil || err != nil {
		return batch, err
	}
	for _, val := range batch.Values() {
		val := o.expr.Eval(val)
		if val.IsQuiet() {
			continue
		}
		select {
		case o.debugCh <- val.Copy():
		case <-o.rctx.Done():
			return nil, o.rctx.Err()
		}
	}
	return batch, err
}
