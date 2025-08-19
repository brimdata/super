package traverse

import (
	"context"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/zbuf"
)

type CachedSubquery struct {
	rctx   *runtime.Context
	body   zbuf.Puller
	cached *super.Value
}

func NewCachedSubquery(rctx *runtime.Context, body zbuf.Puller) *CachedSubquery {
	return &CachedSubquery{rctx: rctx, body: body}
}

func (c *CachedSubquery) Eval(_ super.Value) super.Value {
	if c.cached == nil {
		c.cached = c.exec().Ptr()
	}
	return *c.cached
}

func (c *CachedSubquery) exec() super.Value {
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

// Subquery is a simple subquery mechanism where it has both an Eval
// method to implement expressions and a Pull method to act as the parent
// of a subgraph that is embedded in an expression.  Whenever eval
// is called, it constructs a single valued batch using the passed-in
// this, posts that batch to the embedded query, then pulls from the
// query until eos.
type Subquery struct {
	ctx     context.Context
	sctx    *super.Context
	batchCh chan zbuf.Batch
	eos     bool

	body zbuf.Puller
}

func NewSubquery(rctx *runtime.Context) *Subquery {
	return &Subquery{
		ctx:     rctx.Context,
		sctx:    rctx.Sctx,
		batchCh: make(chan zbuf.Batch, 1),
	}
}

func (s *Subquery) SetBody(body zbuf.Puller) {
	s.body = body
}

func (s *Subquery) Pull(done bool) (zbuf.Batch, error) {
	if s.eos {
		s.eos = false
		return nil, nil
	}
	s.eos = true
	select {
	case batch := <-s.batchCh:
		return batch, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (q *Subquery) Eval(this super.Value) super.Value {
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
