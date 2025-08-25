package switcher

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/sbuf"
)

type Selector struct {
	*op.Router
	expr.Resetter
	cases []*switchCase
}

var _ op.Selector = (*Selector)(nil)

type switchCase struct {
	filter expr.Evaluator
	route  sbuf.Puller
	vals   []super.Value
}

func New(rctx *runtime.Context, parent sbuf.Puller, resetter expr.Resetter) *Selector {
	router := op.NewRouter(rctx, parent)
	s := &Selector{
		Router:   router,
		Resetter: resetter,
	}
	router.Link(s)
	return s
}

func (s *Selector) AddCase(f expr.Evaluator) sbuf.Puller {
	route := s.Router.AddRoute()
	s.cases = append(s.cases, &switchCase{filter: f, route: route})
	return route
}

func (s *Selector) Forward(router *op.Router, batch sbuf.Batch) bool {
	vals := batch.Values()
	for i := range vals {
		this := vals[i]
		for _, c := range s.cases {
			val := c.filter.Eval(this)
			if val.IsMissing() {
				continue
			}
			if val.IsError() {
				// XXX should use structured here to wrap
				// the input value with the error
				c.vals = append(c.vals, val)
				continue
				//XXX don't break here?
				//break
			}
			if val.Type() == super.TypeBool && val.Bool() {
				c.vals = append(c.vals, this)
				break
			}
		}
	}
	// Send each case that has vals from this batch.
	// We have vals that point into the current batch so we
	// ref the batch for each outgoing new batch.
	for _, c := range s.cases {
		if len(c.vals) > 0 {
			// XXX The new slice should come from the
			// outgoing batch so we don't send these slices
			// through GC.
			batch.Ref()
			out := sbuf.NewBatch(c.vals)
			c.vals = nil
			if ok := router.Send(c.route, out, nil); !ok {
				return false
			}
		}
	}
	return true
}
