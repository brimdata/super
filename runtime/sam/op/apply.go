package op

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zbuf"
)

type applier struct {
	rctx     *runtime.Context
	parent   zbuf.Puller
	expr     expr.Evaluator
	resetter expr.Resetter
}

func NewApplier(rctx *runtime.Context, parent zbuf.Puller, expr expr.Evaluator, resetter expr.Resetter) *applier {
	return &applier{
		rctx:     rctx,
		parent:   parent,
		expr:     expr,
		resetter: resetter,
	}
}

func (a *applier) Pull(done bool) (zbuf.Batch, error) {
	for {
		batch, err := a.parent.Pull(done)
		if batch == nil || err != nil {
			a.resetter.Reset()
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
			return zbuf.NewBatch(out), nil
		}
		batch.Unref()
	}
}
