package aggregate

import (
	"github.com/RoaringBitmap/roaring/v2"
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

type Aggregate struct {
	parent vio.Puller
	sctx   *super.Context
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

func New(parent vio.Puller, sctx *super.Context, aggNames []field.Path, aggExprs []expr.Evaluator, aggs []*expr.Aggregator, keyNames []field.Path, keyExprs []expr.Evaluator, partialsIn, partialsOut bool) (*Aggregate, error) {
	builder, err := vector.NewRecordBuilder(sctx, append(keyNames, aggNames...))
	if err != nil {
		return nil, err
	}
	return &Aggregate{
		parent:      parent,
		sctx:        sctx,
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
		vector.Apply(true, func(args ...vector.Any) vector.Any {
			a.consume(args[:len(keys)], args[len(keys):])
			// XXX Perhaps there should be a "consume" version of Apply where
			// no return value is expected.
			return vector.NewNull(args[0].Len())
		}, append(keys, vals...)...)
	}
}

func (a *Aggregate) consume(keys []vector.Any, vals []vector.Any) {
	keys, vals, ok := removeQuietRows(keys, vals)
	if !ok {
		return
	}
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

// removeQuietRows removes rows in which any key is error("quiet").  It returns
// false if all rows are removed.
func removeQuietRows(keys, vals []vector.Any) ([]vector.Any, []vector.Any, bool) {
	if index, ok := notQuietIndex(keys...); ok {
		if len(index) == 0 {
			// All slots are quiet.
			return nil, nil, false
		}
		for i, k := range keys {
			keys[i] = vector.Pick(k, index)
		}
		for i, v := range vals {
			vals[i] = vector.Pick(v, index)
		}
	}
	return keys, vals, true
}

// notQuietIndex returns the slots that are not quiet across vecs (i.e., the
// slot's value is not error("quiet") in any of vecs).  It returns nil, true if
// all slots are quiet and false if no slots are quiet.
func notQuietIndex(vecs ...vector.Any) ([]uint32, bool) {
	rb := quietBitmap(vecs...)
	if rb.IsEmpty() {
		// No slots are quiet.
		return nil, false
	}
	len := uint64(vecs[0].Len())
	if rb.GetCardinality() == len {
		// All slots are quiet.
		return nil, true
	}
	rb.Flip(0, len)
	return rb.ToArray(), true
}

// quietBitmap returns a bitmap in which the bit for each slot is set if the
// slot's value is error("quiet") in any of vecs.
func quietBitmap(vecs ...vector.Any) *roaring.Bitmap {
	var rb roaring.Bitmap
	for _, vec := range vecs {
		errVec, ok := vec.(*vector.Error)
		if !ok || errVec.Vals.Kind() != vector.KindString {
			continue
		}
		valsVec := errVec.Vals
		if _, ok := valsVec.(*vector.Const); ok {
			if vector.StringValue(valsVec, 0) == string(super.Quiet) {
				// Every slot is error("quiet").
				rb.AddRange(0, uint64(valsVec.Len()))
				return &rb
			}
			continue
		}
		for i := range valsVec.Len() {
			if vector.StringValue(valsVec, i) == string(super.Quiet) {
				rb.Add(i)
			}
		}
	}
	return &rb
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
		sctx:        a.sctx,
	}
}

func (a *Aggregate) isCountByString(keyTypes []super.Type) bool {
	return len(a.aggs) == 1 && a.aggs[0].Name == "count" && !a.aggs[0].Distinct &&
		len(keyTypes) == 1 && keyTypes[0].ID() == super.IDString
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
