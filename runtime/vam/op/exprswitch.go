package op

import (
	"context"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type ExprSwitch struct {
	expr   expr.Evaluator
	router *router

	builder      zcode.Builder
	cases        map[string]*route
	caseIndexes  map[*route][]uint32
	defaultRoute *route
}

func NewExprSwitch(ctx context.Context, parent vector.Puller, e expr.Evaluator) *ExprSwitch {
	s := &ExprSwitch{expr: e, cases: map[string]*route{}, caseIndexes: map[*route][]uint32{}}
	s.router = newRouter(ctx, s, parent)
	return s
}

func (s *ExprSwitch) AddCase(val *super.Value) vector.Puller {
	r := s.router.addRoute()
	if val == nil {
		s.defaultRoute = r
	} else {
		s.cases[string(val.Bytes())] = r
	}
	return r
}

func (s *ExprSwitch) forward(vec vector.Any) bool {
	defer clear(s.caseIndexes)
	exprVec := s.expr.Eval(vec)
	for i := range exprVec.Len() {
		s.builder.Truncate()
		exprVec.Serialize(&s.builder, i)
		route, ok := s.cases[string(s.builder.Bytes().Body())]
		if !ok {
			route = s.defaultRoute
		}
		if route != nil {
			s.caseIndexes[route] = append(s.caseIndexes[route], i)
		}
	}
	for route, index := range s.caseIndexes {
		if !route.send(vector.Pick(vec, index), nil) {
			return false
		}
	}
	return true
}
