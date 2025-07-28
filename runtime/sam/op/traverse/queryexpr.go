package traverse

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/zbuf"
)

type QueryExpr struct {
	rctx   *runtime.Context
	puller zbuf.Puller
	cached *super.Value
}

func NewQueryExpr(rctx *runtime.Context, puller zbuf.Puller) *QueryExpr {
	return &QueryExpr{rctx: rctx, puller: puller}
}

func (q *QueryExpr) Eval(this super.Value) super.Value {
	if q.cached == nil {
		q.cached = q.exec().Ptr()
	}
	return *q.cached
}

func (q *QueryExpr) exec() super.Value {
	var batches []zbuf.Batch
	for {
		batch, err := q.puller.Pull(false)
		if err != nil {
			return q.rctx.Sctx.NewError(err)
		}
		if batch == nil {
			return combine(q.rctx.Sctx, batches)
		}
		batches = append(batches, batch)
	}
}
