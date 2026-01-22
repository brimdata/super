package aggregate

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sbuf"
)

type scalarAggregate struct {
	sctx        *super.Context
	parent      sbuf.Puller
	builder     *super.RecordBuilder
	aggRefs     []expr.Evaluator
	aggs        []*expr.Aggregator
	partialsIn  bool
	partialsOut bool
	row         valRow
}

func NewScalar(rctx *runtime.Context, parent sbuf.Puller, aggNames []field.Path, aggs []*expr.Aggregator, partialsIn, partialsOut bool) (sbuf.Puller, error) {
	builder, err := super.NewRecordBuilder(rctx.Sctx, aggNames)
	if err != nil {
		return nil, err
	}
	aggRefs := make([]expr.Evaluator, 0, len(aggNames))
	for _, fieldName := range aggNames {
		aggRefs = append(aggRefs, expr.NewDottedExpr(rctx.Sctx, fieldName))
	}
	return &scalarAggregate{
		sctx:        rctx.Sctx,
		parent:      parent,
		builder:     builder,
		aggRefs:     aggRefs,
		aggs:        aggs,
		partialsIn:  partialsIn,
		partialsOut: partialsOut,
		row:         newValRow(aggs),
	}, nil
}

func (s *scalarAggregate) Pull(done bool) (sbuf.Batch, error) {
	if done {
		s.row = nil
		return nil, nil
	}
	if s.row == nil {
		s.row = newValRow(s.aggs)
		return nil, nil
	}
	for {
		batch, err := s.parent.Pull(false)
		if err != nil {
			return nil, err
		}
		if batch == nil {
			return s.result(), nil
		}
		for _, val := range batch.Values() {
			if s.partialsIn {
				s.row.consumeAsPartial(val, s.aggRefs)
			} else {
				s.row.apply(s.sctx, s.aggs, val)
			}
		}
	}
}

func (s *scalarAggregate) result() sbuf.Batch {
	var typs []super.Type
	s.builder.Reset()
	for _, agg := range s.row {
		var val super.Value
		if s.partialsOut {
			val = agg.ResultAsPartial(s.sctx)
		} else {
			val = agg.Result(s.sctx)
		}
		typs = append(typs, val.Type())
		s.builder.Append(val.Bytes())
	}
	s.row = nil
	typ := s.builder.Type(typs)
	b, err := s.builder.Encode()
	if err != nil {
		panic(err)
	}
	return sbuf.NewBatch([]super.Value{super.NewValue(typ, b)})
}
