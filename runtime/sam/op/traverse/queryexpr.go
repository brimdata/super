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
		q.cached = pullitMakeIt(q.rctx.Sctx, q.puller).Ptr()
	}
	return *q.cached

}

func pullitMakeIt(sctx *super.Context, puller zbuf.Puller) super.Value {
	var vals []super.Value
	for {
		batch, err := puller.Pull(false)
		if err != nil {
			return sctx.NewError(err)
		}
		if batch == nil {
			break
		}
		vals = append(vals, batch.Values()...)
	}
	if len(vals) == 0 {
		return super.Null
	}
	if len(vals) == 1 {
		return vals[0]
	}
	// XXX
	panic("todo")
}
