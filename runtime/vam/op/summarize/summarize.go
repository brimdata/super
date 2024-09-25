package summarize

import (
	zed "github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Summarize struct {
	parent vector.Puller
	zctx   *zed.Context
	// XX Abstract this runtime into a generic table computation.
	// Then the generic interface can execute fast paths for simple scenarios.
	aggs      []*expr.Aggregator
	aggNames  field.List
	keyExprs  []expr.Evaluator
	keyNames  field.List
	typeTable *zed.TypeVectorTable
	builder   *vector.RecordBuilder

	types   []zed.Type
	tables  map[int]aggTable
	results []aggTable
}

func New(parent vector.Puller, zctx *zed.Context, aggNames field.List, aggs []*expr.Aggregator, keyNames []field.Path, keyExprs []expr.Evaluator) (*Summarize, error) {
	builder, err := vector.NewRecordBuilder(zctx, append(keyNames, aggNames...))
	if err != nil {
		return nil, err
	}
	return &Summarize{
		parent:    parent,
		aggs:      aggs,
		keyExprs:  keyExprs,
		tables:    make(map[int]aggTable),
		typeTable: zed.NewTypeVectorTable(),
		types:     make([]zed.Type, len(keyExprs)),
		builder:   builder,
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
			s.tables = nil
			return s.next(), nil
		}
		var keys, vals []vector.Any
		for _, e := range s.keyExprs {
			keys = append(keys, e.Eval(vec))
		}
		for _, e := range s.aggs {
			vals = append(vals, e.Eval(vec))
		}
		vector.Apply(false, func(args ...vector.Any) vector.Any {
			s.consume(args[:len(keys)], args[len(keys):])
			// XXX Perhaps there should be a "consume" version of Apply where
			// no return value is expected.
			return vector.NewConst(zed.Null, args[0].Len(), nil)
		}, append(keys, vals...)...)
	}
}

func (s *Summarize) consume(keys []vector.Any, vals []vector.Any) {
	var keyTypes []zed.Type
	for _, k := range keys {
		keyTypes = append(keyTypes, k.Type())
	}
	tableID := s.typeTable.Lookup(keyTypes)
	table, ok := s.tables[tableID]
	if !ok {
		table = s.newAggTable()
		s.tables[tableID] = table
	}
	table.update(keys, vals)
}

func (s *Summarize) newAggTable() aggTable {
	return &superTable{
		table:   make(map[string]aggRow),
		aggs:    s.aggs,
		builder: s.builder,
	}
}

func (s *Summarize) next() vector.Any {
	if len(s.results) == 0 {
		return nil
	}
	t := s.results[0]
	s.results = s.results[1:]
	return t.materialize()
}
