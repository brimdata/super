package op

import (
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/vam"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/vector"
)

type Debug struct {
	parent vector.Puller
	rctx   *runtime.Context
	expr   expr.Evaluator
	ch     chan sbuf.Batch
}

func NewDebug(rctx *runtime.Context, expr expr.Evaluator, parent vector.Puller) (*Debug, <-chan sbuf.Batch) {
	ch := make(chan sbuf.Batch)
	return &Debug{
		parent: parent,
		rctx:   rctx,
		expr:   expr,
		ch:     ch,
	}, ch
}

func (d *Debug) Pull(done bool) (vector.Any, error) {
	val, err := d.parent.Pull(done)
	if val == nil {
		return nil, err
	}
	if debug := vam.Materialize(d.expr.Eval(val)); len(debug.Values()) != 0 {
		select {
		case d.ch <- debug:
		case <-d.rctx.Done():
			return nil, d.rctx.Err()
		}
	}
	return val, err
}
