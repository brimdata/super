package op

import (
	"context"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

// Subquery is a subquery mechanism with an Eval method to act as an expression
// and a Pull method to act as the parent of the embedded query that implements
// the expression.  When Eval is called, it repeatedly constructs a single-slot
// vector from the current slot of the this vector, sends it to the embedded
// query, pulls from the query until EOS, appends the value to the result
// vector, and then advances to the next slot.
type Subquery struct {
	ctx    context.Context
	sctx   *super.Context
	create func() *Subquery

	body vio.Puller
	ch   chan vector.Any
	eos  bool

	stack []*Subquery
	tos   int

	builder scode.Builder
}

func NewSubquery(ctx context.Context, sctx *super.Context, create func() *Subquery) *Subquery {
	return &Subquery{
		ctx:    ctx,
		sctx:   sctx,
		create: create,
		ch:     make(chan vector.Any, 1),
		tos:    -2,
	}
}

func (s *Subquery) SetBody(body vio.Puller) {
	s.body = body
}

func (s *Subquery) Eval(this vector.Any) vector.Any {
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
		if s.tos >= 10000 {
			return vector.NewWrappedError(s.sctx, "subquery recursion depth exceeded", this)
		}
		if s.tos >= len(s.stack) {
			s.stack = append(s.stack, s.create())
		}
		return s.stack[s.tos].Eval(this)
	}
	db := vector.NewDynamicBuilder()
	index := make([]uint32, this.Len())
	for i := range this.Len() {
		index[i] = i
		select {
		case s.ch <- vector.Pick(this, index[i:i+1]):
		case <-s.ctx.Done():
			msg := s.ctx.Err().Error()
			return vector.NewStringError(s.sctx, msg, this.Len())
		}
		db.Write(s.bodyPull())
	}
	return db.Build(s.sctx)
}

func (s *Subquery) bodyPull() super.Value {
	vec, err := s.body.Pull(false)
	if err != nil {
		return s.sctx.NewError(err)
	}
	if vec == nil {
		return super.Null
	}
	vec2, err := s.body.Pull(false)
	if err != nil {
		return s.sctx.NewError(err)
	}
	if vec2 != nil {
		if _, err := s.body.Pull(true); err != nil {
			return s.sctx.NewError(err)
		}
	}
	if vec2 != nil || vec.Len() > 1 {
		return s.sctx.NewErrorf("query expression produced multiple values (consider [subquery])")
	}
	return vector.ValueAt(&s.builder, vec, 0)
}

func (s *Subquery) Pull(done bool) (vector.Any, error) {
	if s.eos {
		s.eos = false
		return nil, nil
	}
	s.eos = true
	select {
	case vec := <-s.ch:
		return vec, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}
