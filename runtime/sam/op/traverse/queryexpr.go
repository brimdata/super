package traverse

import (
	"context"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zbuf"
)

type Cache struct {
	rctx   *runtime.Context
	body   zbuf.Puller
	cached *super.Value
}

func NewCache(rctx *runtime.Context, body zbuf.Puller) *Cache {
	return &Cache{rctx: rctx, body: body}
}

func (c *Cache) Eval(_ super.Value) super.Value {
	if c.cached == nil {
		c.cached = c.exec().Ptr()
	}
	return *c.cached
}

func (c *Cache) exec() super.Value {
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
// query until eos.  When the subquery is not correlated (i.e., because
// of a new from clause or perhaps a constant values clase) then it wraps
// the from in a cache that always returns the same batch.  Subqueries always
// return single values expecting multi-valued results to be wrapped in a collect.
// If more than one value is returned, then a structured error results.
type QueryExpr struct {
	ctx     context.Context
	sctx    *super.Context
	batchCh chan zbuf.Batch
	eos     bool

	body     zbuf.Puller
	resetter expr.Resetter //XXX why this?
}

func NewQueryExpr(rctx *runtime.Context, resetter expr.Resetter) *QueryExpr {
	return &QueryExpr{
		ctx:      rctx.Context,
		sctx:     rctx.Sctx,
		batchCh:  make(chan zbuf.Batch, 1),
		resetter: resetter,
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
	b := zbuf.NewArray([]super.Value{this})
	select {
	case q.batchCh <- b:
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
				return q.sctx.WrapError(fmt.Sprintf("encountered %d values in expression subquery (consider collect())", count), val)
			}
			return val
		}
		if count == 0 {
			val = b.Values()[0]
		}
		count += len(b.Values())
	}
}
