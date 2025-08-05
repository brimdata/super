package traverse

import (
	"context"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/zbuf"
)

type CachedQueryExpr struct {
	rctx   *runtime.Context
	body   zbuf.Puller
	cached *super.Value
}

func NewCachedQueryExpr(rctx *runtime.Context, body zbuf.Puller) *CachedQueryExpr {
	return &CachedQueryExpr{rctx: rctx, body: body}
}

func (c *CachedQueryExpr) Eval(_ super.Value) super.Value {
	if c.cached == nil {
		c.cached = c.exec().Ptr()
	}
	return *c.cached
}

func (c *CachedQueryExpr) exec() super.Value {
	var batches []zbuf.Batch
	for {
		batch, err := c.body.Pull(false)
		if err != nil {
			return c.rctx.Sctx.NewError(err)
		}
		if batch == nil {
			return combine(c.rctx.Sctx, batches)
		}
		batches = append(batches, batch)
	}
}

// QueryExpr is a simple subquery mechanism where it has both an Eval
// method to implement expressions and a Pull method to act as the parent
// of a subgraph that is embedded in an expression.  Whenever eval
// is called, it constructs a single valued batch using the passed-in
// this, posts that batch to the embedded query, then pulls from the
// query until eos.
type QueryExpr struct {
	ctx     context.Context
	sctx    *super.Context
	batchCh chan zbuf.Batch
	eos     bool

	body zbuf.Puller
}

func NewQueryExpr(rctx *runtime.Context) *QueryExpr {
	return &QueryExpr{
		ctx:     rctx.Context,
		sctx:    rctx.Sctx,
		batchCh: make(chan zbuf.Batch, 1),
	}
}

func (q *QueryExpr) SetBody(body zbuf.Puller) {
	q.body = body
}

func (q *QueryExpr) Pull(done bool) (zbuf.Batch, error) {
	if q.eos {
		q.eos = false
		return nil, nil
	}
	q.eos = true
	select {
	case batch := <-q.batchCh:
		return batch, nil
	case <-q.ctx.Done():
		return nil, q.ctx.Err()
	}
}

func (q *QueryExpr) Eval(this super.Value) super.Value {
	select {
	case q.batchCh <- zbuf.NewArray([]super.Value{this}):
	case <-q.ctx.Done():
		return q.sctx.NewError(q.ctx.Err())
	}
	val := super.Null
	var count int
	for {
		b, err := q.body.Pull(false)
		if err != nil {
			panic(err)
		}
		if b == nil {
			if count > 1 {
				return q.sctx.NewErrorf("query expression produced multiple values (consider collect())")
			}
			return val
		}
		if count == 0 {
			val = b.Values()[0].Copy()
		}
		count += len(b.Values())
		b.Unref()
	}
}
