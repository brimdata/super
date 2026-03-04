package op

import (
	"context"

	"github.com/brimdata/super/runtime/vam"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/vector"
)

type Debug struct {
	parent vector.Puller
	ctx    context.Context
	expr   expr.Evaluator
	filter expr.Evaluator
	ch     chan sbuf.Batch
}

func NewDebug(ctx context.Context, expr expr.Evaluator, filter expr.Evaluator, parent vector.Puller) (*Debug, <-chan sbuf.Batch) {
	ch := make(chan sbuf.Batch)
	return &Debug{
		parent: parent,
		ctx:    ctx,
		expr:   expr,
		filter: filter,
		ch:     ch,
	}, ch
}

func (d *Debug) Pull(done bool) (vector.Any, error) {
	val, err := d.parent.Pull(done)
	if val == nil {
		return nil, err
	}
	filtered := val
	if d.filter != nil {
		filtered, _ = applyMask(val, d.filter.Eval(filtered))
	}
	if debug := vam.Materialize(d.expr.Eval(filtered)); len(debug.Values()) != 0 {
		select {
		case d.ch <- debug:
		case <-d.ctx.Done():
			return nil, d.ctx.Err()
		}
	}
	return val, err
}
