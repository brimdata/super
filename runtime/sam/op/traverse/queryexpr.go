package traverse

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zcode"
)

type QueryExpr struct {
	rctx       *runtime.Context
	puller     zbuf.Puller
	cached     *super.Value
	forceArray bool
}

func NewQueryExpr(rctx *runtime.Context, puller zbuf.Puller, forceArray bool) *QueryExpr {
	return &QueryExpr{rctx: rctx, puller: puller, forceArray: forceArray}
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
			if q.forceArray {
				return arrayResult(q.rctx.Sctx, batches)
			}
			return combine(q.rctx.Sctx, batches)
		}
		batches = append(batches, batch)
	}
}

func arrayResult(sctx *super.Context, batches []zbuf.Batch) super.Value {
	var vals []super.Value
	for _, batch := range batches {
		vals = append(vals, batch.Values()...)
	}
	if len(vals) == 0 {
		typ := sctx.LookupTypeArray(super.TypeNull)
		return super.NewValue(typ, zcode.Bytes{})
	}
	typ := vals[0].Type()
	for _, val := range vals[1:] {
		if typ != val.Type() {
			return makeUnionArray(sctx, vals)
		}
	}
	var b zcode.Builder
	for _, val := range vals {
		b.Append(val.Bytes())
	}
	return super.NewValue(sctx.LookupTypeArray(typ), b.Bytes())
}
