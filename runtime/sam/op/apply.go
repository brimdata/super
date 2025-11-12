package op

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sbuf"
)

type applier struct {
	rctx   *runtime.Context
	parent sbuf.Puller
	expr   expr.Evaluator
}

func NewApplier(rctx *runtime.Context, parent sbuf.Puller, expr expr.Evaluator) *applier {
	return &applier{
		rctx:   rctx,
		parent: parent,
		expr:   expr,
	}
}

func (a *applier) Pull(done bool) (sbuf.Batch, error) {
	for {
		batch, err := a.parent.Pull(done)
		if batch == nil || err != nil {
			return nil, err
		}
		vals := batch.Values()
		out := make([]super.Value, 0, len(vals))
		for i := range vals {
			val := a.expr.Eval(vals[i])
			if val.IsError() {
				if val.IsQuiet() || val.IsMissing() {
					continue
				}
			}
			out = append(out, val)
		}
		if len(out) > 0 {
			return sbuf.NewBatch(out), nil
		}
		batch.Unref()
	}
}
