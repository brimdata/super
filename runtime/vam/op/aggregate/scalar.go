package aggregate

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/runtime/vam/expr/agg"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type scalarAggregate struct {
	parent      vector.Puller
	sctx        *super.Context
	aggExprs    []expr.Evaluator
	aggs        []*expr.Aggregator
	builder     *vector.RecordBuilder
	partialsIn  bool
	partialsOut bool

	funcs []agg.Func
}

func NewScalar(parent vector.Puller, sctx *super.Context, aggs []*expr.Aggregator, aggNames []field.Path, aggExprs []expr.Evaluator, partialsIn, partialsOut bool) (vector.Puller, error) {
	builder, err := vector.NewRecordBuilder(sctx, aggNames)
	if err != nil {
		return nil, err
	}
	return &scalarAggregate{
		parent:      parent,
		sctx:        sctx,
		aggs:        aggs,
		aggExprs:    aggExprs,
		builder:     builder,
		partialsIn:  partialsIn,
		partialsOut: partialsOut,
		funcs:       newFuncs(aggs),
	}, nil
}

func (s *scalarAggregate) Pull(done bool) (vector.Any, error) {
	if s.funcs == nil {
		s.funcs = newFuncs(s.aggs)
		return nil, nil
	}
	for {
		vec, err := s.parent.Pull(done)
		if err != nil {
			return nil, err
		}
		if vec == nil {
			return s.result(), nil
		}
		var vals []vector.Any
		if s.partialsIn {
			for _, e := range s.aggExprs {
				vals = append(vals, e.Eval(vec))
			}
		} else {
			for _, e := range s.aggs {
				vals = append(vals, e.Eval(vec))
			}
		}
		vector.Apply(false, func(vecs ...vector.Any) vector.Any {
			for i, vec := range vecs {
				if s.partialsIn {
					s.funcs[i].ConsumeAsPartial(vec)
				} else {
					s.funcs[i].Consume(vec)
				}
			}
			return vector.NewConst(super.Null, vecs[0].Len(), bitvec.Zero)
		}, vals...)
	}
}

func newFuncs(aggs []*expr.Aggregator) []agg.Func {
	var funcs []agg.Func
	for _, agg := range aggs {
		funcs = append(funcs, agg.Pattern())
	}
	return funcs
}

func (s *scalarAggregate) result() vector.Any {
	var vecs []vector.Any
	for _, f := range s.funcs {
		b := vector.NewDynamicBuilder()
		if s.partialsOut {
			b.Write(f.ResultAsPartial(s.sctx))
		} else {
			b.Write(f.Result(s.sctx))
		}
		vecs = append(vecs, b.Build())
	}
	s.funcs = nil
	return s.builder.New(vecs, bitvec.Zero)
}
