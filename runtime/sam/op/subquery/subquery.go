package subquery

import (
	"context"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/scode"
)

type CachedSubquery struct {
	rctx   *runtime.Context
	body   sbuf.Puller
	cached *super.Value
}

func NewCachedSubquery(rctx *runtime.Context, body sbuf.Puller) *CachedSubquery {
	return &CachedSubquery{rctx: rctx, body: body}
}

func (c *CachedSubquery) Eval(_ super.Value) super.Value {
	if c.cached == nil {
		c.cached = c.exec().Ptr()
	}
	return *c.cached
}

func (c *CachedSubquery) exec() super.Value {
	var batches []sbuf.Batch
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
	batchCh chan sbuf.Batch
	eos     bool
	tos     int
	stack   []*Subquery
	create  func() *Subquery

	body sbuf.Puller
}

func NewSubquery(rctx *runtime.Context, create func() *Subquery) *Subquery {
	return &Subquery{
		ctx:     rctx.Context,
		sctx:    rctx.Sctx,
		tos:     -2,
		create:  create,
		batchCh: make(chan sbuf.Batch, 1),
	}
}

func (s *Subquery) SetBody(body sbuf.Puller) {
	s.body = body
}

func (s *Subquery) Pull(done bool) (sbuf.Batch, error) {
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

const MaxSubqueryRecursion = 10000

func (s *Subquery) Eval(this super.Value) super.Value {
	s.tos++
	defer func() {
		s.tos--
	}()
	if s.tos >= 0 {
		// We're re-entering this subquery instance before it's done evaluating
		// the previous invocation.  This happens when a subquery is invoked
		// inside of a recursive function so the same instance ends up being
		// called by different call frames.  To deal with this, we keep a stack
		// of Subquery duplicates where each duplicate is not shared and extend
		// the stack as needed.  If the stack overflows, we return an error.
		if s.tos >= MaxSubqueryRecursion {
			return s.sctx.WrapError("subquery recursion depth exceeded", this)
		}
		if s.tos >= len(s.stack) {
			s.stack = append(s.stack, s.create())
		}
		return s.stack[s.tos].Eval(this)
	}
	select {
	case s.batchCh <- sbuf.NewArray([]super.Value{this}):
	case <-s.ctx.Done():
		return s.sctx.NewError(s.ctx.Err())
	}
	val := super.Null
	var count int
	for {
		b, err := s.body.Pull(false)
		if err != nil {
			panic(err)
		}
		if b == nil {
			if count > 1 {
				return s.sctx.NewErrorf("query expression produced multiple values (consider [subquery])")
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

func combine(sctx *super.Context, batches []sbuf.Batch) super.Value {
	switch len(batches) {
	case 0:
		return super.Null
	case 1:
		return makeArray(sctx, batches[0].Values())
	default:
		var vals []super.Value
		for _, batch := range batches {
			vals = append(vals, batch.Values()...)
		}
		return makeArray(sctx, vals)
	}
}

func makeArray(sctx *super.Context, vals []super.Value) super.Value {
	if len(vals) == 0 {
		return super.Null
	}
	if len(vals) == 1 {
		return vals[0]
	}
	typ := vals[0].Type()
	for _, val := range vals[1:] {
		if typ != val.Type() {
			return makeUnionArray(sctx, vals)
		}
	}
	var b scode.Builder
	for _, val := range vals {
		b.Append(val.Bytes())
	}
	return super.NewValue(sctx.LookupTypeArray(typ), b.Bytes())
}

func makeUnionArray(sctx *super.Context, vals []super.Value) super.Value {
	types := make(map[super.Type]struct{})
	for _, val := range vals {
		types[val.Type()] = struct{}{}
	}
	utypes := make([]super.Type, 0, len(types))
	for typ := range types {
		utypes = append(utypes, typ)
	}
	union := sctx.LookupTypeUnion(utypes)
	var b scode.Builder
	for _, val := range vals {
		super.BuildUnion(&b, union.TagOf(val.Type()), val.Bytes())
	}
	return super.NewValue(sctx.LookupTypeArray(union), b.Bytes())
}
