package aggregate

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Aggregate struct {
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

func New(parent vector.Puller, zctx *super.Context, aggNames []field.Path, aggExprs []expr.Evaluator, aggs []*expr.Aggregator, keyNames []field.Path, keyExprs []expr.Evaluator, partialsIn, partialsOut bool) (*Aggregate, error) {
	builder, err := vector.NewRecordBuilder(zctx, append(keyNames, aggNames...))
	if err != nil {
		return nil, err
	}
	return &Aggregate{
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

func (a *Aggregate) Pull(done bool) (vector.Any, error) {
	if done {
		_, err := a.parent.Pull(done)
		return nil, err
	}
	if a.results != nil {
		return a.next(), nil
	}
	for {
		//XXX check context Done
		vec, err := a.parent.Pull(false)
		if err != nil {
			return nil, err
		}
		if vec == nil {
			for _, t := range a.tables {
				a.results = append(a.results, t)
			}
			clear(a.tables)
			return a.next(), nil
		}
		var keys, vals []vector.Any
		for _, e := range a.keyExprs {
			keys = append(keys, e.Eval(vec))
		}
		if a.partialsIn {
			for _, e := range a.aggExprs {
				vals = append(vals, e.Eval(vec))
			}
		} else {
			for _, e := range a.aggs {
				vals = append(vals, e.Eval(vec))
			}
		}
		vector.Apply(false, func(args ...vector.Any) vector.Any {
			a.consume(args[:len(keys)], args[len(keys):])
			// XXX Perhaps there should be a "consume" version of Apply where
			// no return value is expected.
			return vector.NewConst(super.Null, args[0].Len(), nil)
		}, append(keys, vals...)...)
	}
}

func (a *Aggregate) consume(keys []vector.Any, vals []vector.Any) {
	var keyTypes []super.Type
	for _, k := range keys {
		keyTypes = append(keyTypes, k.Type())
	}
	tableID := a.typeTable.Lookup(keyTypes)
	table, ok := a.tables[tableID]
	if !ok {
		table = a.newAggTable(keyTypes)
		a.tables[tableID] = table
	}
	table.update(keys, vals)
}

func (a *Aggregate) newAggTable(keyTypes []super.Type) aggTable {
	// Check if we can us an optimized table, else go slow path.
	if a.isCountByString(keyTypes) && len(a.aggs) == 1 && a.aggs[0].Where == nil {
		// countByString.update does not handle nulls in its vals param.
		return newCountByString(a.builder, a.partialsIn)
	}
	return &superTable{
		aggs:        a.aggs,
		builder:     a.builder,
		partialsIn:  a.partialsIn,
		partialsOut: a.partialsOut,
		table:       make(map[string]int),
		zctx:        a.zctx,
	}
}

func (a *Aggregate) isCountByString(keyTypes []super.Type) bool {
	return len(a.aggs) == 1 && len(keyTypes) == 1 && a.aggs[0].Name == "count" &&
		keyTypes[0].ID() == super.IDString
}

func (a *Aggregate) next() vector.Any {
	if len(a.results) == 0 {
		a.results = nil
		return nil
	}
	t := a.results[0]
	a.results = a.results[1:]
	return t.materialize()
}
