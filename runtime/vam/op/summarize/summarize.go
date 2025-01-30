package summarize

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Summarize struct {
	parent vector.Puller
	zctx   *super.Context
	// XX Abstract this runtime into a generic table computation.
	// Then the generic interface can execute fast paths for simple scenarios.
	aggs        []*expr.Aggregator
	aggExprs    []expr.Evaluator
	keyExprs    []expr.Evaluator
	typeTable   *super.TypeVectorTable
	builder     *vector.RecordBuilder
	partialsIn  bool
	partialsOut bool

	types   []super.Type
	tables  map[int]aggTable
	results []aggTable
}

func New(parent vector.Puller, zctx *super.Context, aggNames []field.Path, aggExprs []expr.Evaluator, aggs []*expr.Aggregator, keyNames []field.Path, keyExprs []expr.Evaluator, partialsIn, partialsOut bool) (*Summarize, error) {
	builder, err := vector.NewRecordBuilder(zctx, append(keyNames, aggNames...))
	if err != nil {
		return nil, err
	}
	return &Summarize{
		parent:      parent,
		zctx:        zctx,
		aggs:        aggs,
		aggExprs:    aggExprs,
		keyExprs:    keyExprs,
		tables:      make(map[int]aggTable),
		typeTable:   super.NewTypeVectorTable(),
		types:       make([]super.Type, len(keyExprs)),
		builder:     builder,
		partialsIn:  partialsIn,
		partialsOut: partialsOut,
	}, nil
}

func (s *Summarize) Pull(done bool) (vector.Any, error) {
	if done {
		_, err := s.parent.Pull(done)
		return nil, err
	}
	if s.results != nil {
		return s.next(), nil
	}
	for {
		//XXX check context Done
		vec, err := s.parent.Pull(false)
		if err != nil {
			return nil, err
		}
		if vec == nil {
			for _, t := range s.tables {
				s.results = append(s.results, t)
			}
			clear(s.tables)
			return s.next(), nil
		}
		var keys, vals []vector.Any
		for _, e := range s.keyExprs {
			keys = append(keys, e.Eval(vec))
		}
		if s.partialsIn {
			for _, e := range s.aggExprs {
				vals = append(vals, e.Eval(vec))
			}
		} else {
			for _, e := range s.aggs {
				vals = append(vals, e.Eval(vec))
			}
		}
		vector.Apply(false, func(args ...vector.Any) vector.Any {
			s.consume(args[:len(keys)], args[len(keys):])
			// XXX Perhaps there should be a "consume" version of Apply where
			// no return value is expected.
			return vector.NewConst(super.Null, args[0].Len(), nil)
		}, append(keys, vals...)...)
	}
}

func (s *Summarize) consume(keys []vector.Any, vals []vector.Any) {
	var keyTypes []super.Type
	for _, k := range keys {
		keyTypes = append(keyTypes, k.Type())
	}
	tableID := s.typeTable.Lookup(keyTypes)
	table, ok := s.tables[tableID]
	if !ok {
		table = s.newAggTable(keyTypes)
		s.tables[tableID] = table
	}
	table.update(keys, vals)
}

func (s *Summarize) newAggTable(keyTypes []super.Type) aggTable {
	// Check if we can us an optimized table, else go slow path.
	if s.isCountByString(keyTypes) {
		return newCountByString(s.builder, s.partialsIn)
	}
	return &superTable{
		aggs:        s.aggs,
		builder:     s.builder,
		partialsIn:  s.partialsIn,
		partialsOut: s.partialsOut,
		table:       make(map[string]int),
		zctx:        s.zctx,
	}
}

func (s *Summarize) isCountByString(keyTypes []super.Type) bool {
	return len(s.aggs) == 1 && len(keyTypes) == 1 && s.aggs[0].Name == "count" &&
		keyTypes[0].ID() == super.IDString
}

func (s *Summarize) next() vector.Any {
	if len(s.results) == 0 {
		s.results = nil
		return nil
	}
	t := s.results[0]
	s.results = s.results[1:]
	return t.materialize()
}
