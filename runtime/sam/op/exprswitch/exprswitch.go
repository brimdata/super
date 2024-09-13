package exprswitch

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/runtime"
	"github.com/brimdata/zed/runtime/sam/expr"
	"github.com/brimdata/zed/runtime/sam/op"
	"github.com/brimdata/zed/zbuf"
)

type ExprSwitch struct {
	*op.Router
	expr.Resetter
	expr        expr.Evaluator
	cases       map[string]*switchCase
	defaultCase *switchCase
}

var _ op.Selector = (*ExprSwitch)(nil)

type switchCase struct {
	route zbuf.Puller
	vals  []zed.Value
}

func New(rctx *runtime.Context, parent zbuf.Puller, e expr.Evaluator, resetter expr.Resetter) *ExprSwitch {
	router := op.NewRouter(rctx, parent)
	s := &ExprSwitch{
		Router:   router,
		Resetter: resetter,
		expr:     e,
		cases:    make(map[string]*switchCase),
	}
	router.Link(s)
	return s
}

func (s *ExprSwitch) AddCase(val *zed.Value) zbuf.Puller {
	route := s.Router.AddRoute()
	if val == nil {
		s.defaultCase = &switchCase{route: route}
	} else {
		s.cases[string(val.Bytes())] = &switchCase{route: route}
	}
	return route
}

func (s *ExprSwitch) Forward(router *op.Router, batch zbuf.Batch) bool {
	arena := zed.NewArena()
	defer arena.Unref()
	ectx := expr.NewContextWithVars(arena, batch.Vars())
	vals := batch.Values()
	for i := range vals {
		val := s.expr.Eval(ectx, vals[i])
		if val.IsMissing() {
			continue
		}
		which, ok := s.cases[string(val.Bytes())]
		if !ok {
			which = s.defaultCase
		}
		if which == nil {
			continue
		}
		which.vals = append(which.vals, vals[i])
	}
	// Send each case that has vals from this batch.
	// We have vals that point into the current batch so we
	// ref the batch for each outgoing new batch.
	for _, c := range s.cases {
		if len(c.vals) > 0 {
			out := zbuf.NewBatch(arena, c.vals, batch, batch.Vars())
			c.vals = nil
			if ok := router.Send(c.route, out, nil); !ok {
				return false
			}
		}
	}
	if c := s.defaultCase; c != nil && len(c.vals) > 0 {
		out := zbuf.NewBatch(arena, c.vals, batch, batch.Vars())
		c.vals = nil
		if ok := router.Send(c.route, out, nil); !ok {
			return false
		}
	}
	return true
}
