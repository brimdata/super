package proc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/brimsec/zq/ast"
	"github.com/brimsec/zq/expr"
	"github.com/brimsec/zq/reducer"
	"github.com/brimsec/zq/reducer/compile"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zcode"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
)

type GroupByKey struct {
	target string
	expr   expr.ExpressionEvaluator
}

type GroupByParams struct {
	inputSortDir int
	limit        int
	keys         []GroupByKey
	reducers     []compile.CompiledReducer
	builder      *ColumnBuilder
	consumePart  bool
	emitPart     bool
}

type errTooBig int

func (e errTooBig) Error() string {
	return fmt.Sprintf("non-decomposable groupby aggregation exceeded configured cardinality limit (%d)", e)
}

func IsErrTooBig(err error) bool {
	_, ok := err.(errTooBig)
	return ok
}

var DefaultGroupByLimit = 1000000

func CompileGroupBy(node *ast.GroupByProc, zctx *resolver.Context) (*GroupByParams, error) {
	keys := make([]GroupByKey, 0)
	var targets []string
	for _, astKey := range node.Keys {
		ex, err := compileKeyExpr(astKey.Expr)
		if err != nil {
			return nil, fmt.Errorf("compiling groupby: %w", err)
		}
		keys = append(keys, GroupByKey{
			target: astKey.Target,
			expr:   ex,
		})
		targets = append(targets, astKey.Target)
	}
	reducers := make([]compile.CompiledReducer, 0)
	for _, reducer := range node.Reducers {
		compiled, err := compile.Compile(reducer)
		if err != nil {
			return nil, err
		}
		reducers = append(reducers, compiled)
	}
	builder, err := NewColumnBuilder(zctx, targets)
	if err != nil {
		return nil, fmt.Errorf("compiling groupby: %w", err)
	}
	if (node.ConsumePart || node.EmitPart) && !decomposable(reducers) {
		return nil, errors.New("partial input or output requested with non-decomposable reducers")
	}
	return &GroupByParams{
		limit:        node.Limit,
		keys:         keys,
		reducers:     reducers,
		builder:      builder,
		inputSortDir: node.InputSortDir,
		consumePart:  node.ConsumePart,
		emitPart:     node.EmitPart,
	}, nil
}

func compileKeyExpr(ex ast.Expression) (expr.ExpressionEvaluator, error) {
	if fe, ok := ex.(ast.FieldExpr); ok {
		f, err := expr.CompileFieldExpr(fe)
		if err != nil {
			return nil, err
		}
		ev := func(r *zng.Record) (zng.Value, error) {
			return f(r), nil
		}
		return ev, nil
	}
	return expr.CompileExpr(ex)
}

// GroupBy computes aggregations using a GroupByAggregator.
type GroupBy struct {
	Base
	agg      *GroupByAggregator
	once     sync.Once
	resultCh chan Result
}

// A keyRow holds information about the key column types that result
// from a given incoming type ID.
type keyRow struct {
	id      int
	columns []zng.Column
}

// GroupByAggregator performs the core aggregation computation for a
// list of reducer generators. It handles both regular and time-binned
// ("every") group-by operations.  Records are generated in a
// deterministic but undefined total order.
type GroupByAggregator struct {
	// keyRows maps incoming type ID to a keyRow holding
	// information on the column types for that record's group-by
	// keys. If the inbound record doesn't have all of the keys,
	// then it is blocked by setting the map entry to nil. If
	// there are no group-by keys, then the map is set to an empty
	// slice.
	keyRows  map[int]keyRow
	cacheKey []byte // Reduces memory allocations in Consume.
	// zctx is the type context of the running search.
	zctx *resolver.Context
	// kctx is a scratch type context used to generate unique
	// type IDs for prepending to the entires for the key-value
	// lookup table so that values with the same encoding but of
	// different types do not collide.  No types from this context
	// are ever referenced.
	kctx         *resolver.Context
	keys         []GroupByKey
	keyResolvers []expr.FieldExprResolver
	decomposable bool
	reducerDefs  []compile.CompiledReducer
	builder      *ColumnBuilder
	table        map[string]*GroupByRow
	limit        int
	valueCompare expr.ValueCompareFn // to compare primary group keys for early key output
	keyCompare   expr.CompareFn      // compare the first key (used when input sorted)
	keysCompare  expr.CompareFn      // compare all keys
	maxTableKey  *zng.Value
	maxSpillKey  *zng.Value
	inputSortDir int
	runManager   *runManager
	consumePart  bool
	emitPart     bool
}

type GroupByRow struct {
	keycols  []zng.Column
	keyvals  zcode.Bytes
	groupval *zng.Value // for sorting when input sorted
	reducers compile.Row
}

func NewGroupByAggregator(c *Context, params GroupByParams) *GroupByAggregator {
	limit := params.limit
	if limit == 0 {
		limit = DefaultGroupByLimit
	}
	var valueCompare expr.ValueCompareFn
	var keyCompare, keysCompare expr.CompareFn

	if len(params.keys) > 0 && params.inputSortDir != 0 {
		// As the default sort behavior, nullsMax=true is also expected for streaming groupby.
		vs := expr.NewValueCompareFn(true)
		if params.inputSortDir < 0 {
			valueCompare = func(a, b zng.Value) int { return vs(b, a) }
		} else {
			valueCompare = vs
		}
		rs := expr.NewCompareFn(true, expr.CompileFieldAccess(params.keys[0].target))
		if params.inputSortDir < 0 {
			keyCompare = func(a, b *zng.Record) int { return rs(b, a) }
		} else {
			keyCompare = rs
		}
	}
	var resolvers []expr.FieldExprResolver
	for _, k := range params.keys {
		resolvers = append(resolvers, expr.CompileFieldAccess(k.target))
	}
	rs := expr.NewCompareFn(true, resolvers...)
	if params.inputSortDir < 0 {
		keysCompare = func(a, b *zng.Record) int { return rs(b, a) }
	} else {
		keysCompare = rs
	}
	return &GroupByAggregator{
		inputSortDir: params.inputSortDir,
		limit:        limit,
		keys:         params.keys,
		keyResolvers: resolvers,
		zctx:         c.TypeContext,
		kctx:         resolver.NewContext(),
		decomposable: decomposable(params.reducers),
		reducerDefs:  params.reducers,
		builder:      params.builder,
		keyRows:      make(map[int]keyRow),
		table:        make(map[string]*GroupByRow),
		keyCompare:   keyCompare,
		keysCompare:  keysCompare,
		valueCompare: valueCompare,
		consumePart:  params.consumePart,
		emitPart:     params.emitPart,
	}
}

func decomposable(rs []compile.CompiledReducer) bool {
	for _, r := range rs {
		instance := r.Instantiate()
		if _, ok := instance.(reducer.Decomposable); !ok {
			return false
		}
	}
	return true
}

func NewGroupBy(c *Context, parent Proc, params GroupByParams) *GroupBy {
	// XXX in a subsequent PR we will isolate ast params and pass in
	// ast.GroupByParams
	agg := NewGroupByAggregator(c, params)
	return &GroupBy{
		Base:     Base{Context: c, Parent: parent},
		agg:      agg,
		resultCh: make(chan Result),
	}
}

func (g *GroupBy) Pull() (zbuf.Batch, error) {
	g.once.Do(func() { go g.run() })
	if r, ok := <-g.resultCh; ok {
		return r.Batch, r.Err
	}
	return nil, g.Context.Err()
}

func (g *GroupBy) run() {
	defer func() {
		close(g.resultCh)
		if g.agg.runManager != nil {
			g.agg.runManager.cleanup()
		}
	}()
	for {
		batch, err := g.Get()
		if err != nil {
			g.sendResult(nil, err)
			return
		}
		if batch == nil {
			for {
				b, err := g.agg.Results(true)
				g.sendResult(b, err)
				if b == nil {
					return
				}
			}
		}
		for k := 0; k < batch.Length(); k++ {
			if err := g.agg.Consume(batch.Index(k)); err != nil {
				batch.Unref()
				g.sendResult(nil, err)
				return
			}
		}
		batch.Unref()
		if g.agg.inputSortDir == 0 {
			continue
		}
		// sorted input: see if we have any completed keys we can emit.
		for {
			res, err := g.agg.Results(false)
			if err != nil {
				g.sendResult(nil, err)
				return
			}
			if res == nil {
				break
			}
			expr.SortStable(res.Records(), g.agg.keyCompare)
			g.sendResult(res, nil)
		}
	}
}

func (g *GroupBy) sendResult(b zbuf.Batch, err error) {
	select {
	case g.resultCh <- Result{Batch: b, Err: err}:
	case <-g.Context.Done():
	}
}

func (g *GroupByAggregator) createGroupByRow(keyCols []zng.Column, vals zcode.Bytes, groupval *zng.Value) *GroupByRow {
	// Make a deep copy so the caller can reuse the underlying arrays.
	v := make(zcode.Bytes, len(vals))
	copy(v, vals)
	return &GroupByRow{
		keycols:  keyCols,
		keyvals:  v,
		groupval: groupval,
		reducers: compile.NewRow(g.reducerDefs),
	}
}

func newKeyRow(kctx *resolver.Context, r *zng.Record, keys []GroupByKey) (keyRow, error) {
	cols := make([]zng.Column, len(keys))
	for k, key := range keys {
		keyVal, err := key.expr(r)
		// Don't err on ErrNoSuchField; just return an empty
		// keyRow and the descriptor will be blocked.
		if err != nil && !errors.Is(err, expr.ErrNoSuchField) {
			return keyRow{}, err
		}
		if keyVal.Type == nil {
			return keyRow{}, nil
		}
		cols[k] = zng.NewColumn(key.target, keyVal.Type)
	}
	// Lookup a unique ID by converting the columns too a record string
	// and looking up the record by name in the scratch type context.
	// This is called infrequently, just once for each unique input
	// record type.  If there no keys, just use id zero since the
	// type ID doesn't matter here.
	var id int
	if len(cols) > 0 {
		typ, err := kctx.LookupTypeRecord(cols)
		if err != nil {
			return keyRow{}, err
		}
		id = typ.ID()
	}
	return keyRow{id, cols}, nil
}

// Consume adds a record to the aggregation.
func (g *GroupByAggregator) Consume(r *zng.Record) error {
	// First check if we've seen this descriptor before and if not
	// build an entry for it.
	id := r.Type.ID()
	keyRow, ok := g.keyRows[id]
	if !ok {
		var err error
		keyRow, err = newKeyRow(g.kctx, r, g.keys)
		if err != nil {
			return err
		}
		g.keyRows[id] = keyRow
	}

	if keyRow.columns == nil {
		// block this descriptor since it doesn't have all the group-by keys
		return nil
	}

	// See if we've encountered this row before.
	// We compute a key for this row by exploiting the fact that
	// a row key is uniquely determined by the inbound descriptor
	// (implying the types of the keys) and the keys values.
	// We don't know the reducer types ahead of time so we can't compute
	// the final desciptor yet, but it doesn't matter.  Note that a given
	// input descriptor may end up with multiple output descriptors
	// (because the reducer types are different for the same keys), but
	// because our goal is to distingush rows for different types of keys,
	// we can rely on just the key types (and input desciptor uniquely
	// implying those types)

	var keyBytes zcode.Bytes
	if g.cacheKey != nil {
		keyBytes = g.cacheKey[:4]
	} else {
		keyBytes = make(zcode.Bytes, 4, 128)
	}
	binary.BigEndian.PutUint32(keyBytes, uint32(keyRow.id))
	g.builder.Reset()
	var prim *zng.Value
	for i, key := range g.keys {
		keyVal, err := key.expr(r)
		if err != nil && !errors.Is(err, zng.ErrUnset) {
			return err
		}
		if i == 0 && g.inputSortDir != 0 {
			g.updateMaxTableKey(keyVal)
			prim = &keyVal
		}
		g.builder.Append(keyVal.Bytes, keyVal.IsContainer())
	}
	zv, err := g.builder.Encode()
	if err != nil {
		// XXX internal error
	}
	keyBytes = append(keyBytes, zv...)
	g.cacheKey = keyBytes

	row, ok := g.table[string(keyBytes)]
	if !ok {
		if len(g.table) >= g.limit {
			if !g.decomposable {
				return errTooBig(g.limit)
			}
			if err := g.spillTable(false); err != nil {
				return err
			}
		}
		row = g.createGroupByRow(keyRow.columns, keyBytes[4:], prim)
		g.table[string(keyBytes)] = row
	}

	if g.consumePart {
		return row.reducers.ConsumePart(r)
	}
	row.reducers.Consume(r)
	return nil
}

func (g *GroupByAggregator) spillTable(eof bool) error {
	batch, err := g.readTable(true, true)
	if err != nil || batch == nil {
		return err
	}
	if g.runManager == nil {
		g.runManager, err = newRunManager(g.keysCompare)
		if err != nil {
			return err
		}
	}
	recs := batch.Records()
	// Note that this will sort recs according to g.keysCompare.
	if err := g.runManager.createRun(recs); err != nil {
		return err
	}
	if !eof && g.inputSortDir != 0 {
		v, err := g.keys[0].expr(recs[len(recs)-1])
		if err != nil && !errors.Is(err, zng.ErrUnset) {
			return err
		}
		g.updateMaxSpillKey(v)
	}
	return nil
}

func (g *GroupByAggregator) updateMaxTableKey(v zng.Value) {
	if g.maxTableKey == nil {
		g.maxTableKey = &v
		return
	}
	if g.valueCompare(v, *g.maxTableKey) > 0 {
		g.maxTableKey = &v
	}
}

func (g *GroupByAggregator) updateMaxSpillKey(v zng.Value) {
	if g.maxSpillKey == nil {
		g.maxSpillKey = &v
		return
	}
	if g.valueCompare(v, *g.maxSpillKey) > 0 {
		g.maxSpillKey = &v
	}
}

// Results returns a batch of aggregation result records. Upon eof,
// this should be called repeatedly until a nil batch is returned. If
// the input is sorted in the primary key, Results can be called
// before eof, and keys that are completed will returned.
func (g *GroupByAggregator) Results(eof bool) (zbuf.Batch, error) {
	if g.runManager == nil {
		return g.readTable(eof, g.emitPart)
	}
	if eof {
		// EOF: spill in-memory table before merging all files for output.
		if err := g.spillTable(true); err != nil {
			return nil, err
		}
	}
	return g.readSpills(eof)
}

const batchLen = 100 // like sort

func (g *GroupByAggregator) readSpills(eof bool) (zbuf.Batch, error) {
	recs := make([]*zng.Record, 0, batchLen)
	if !eof && g.inputSortDir == 0 {
		return nil, nil
	}
	for len(recs) < batchLen {
		if !eof && g.inputSortDir != 0 {
			rec, err := g.runManager.Peek()
			if err != nil {
				return nil, err
			}
			if rec == nil {
				break
			}
			keyVal, err := g.keys[0].expr(rec)
			if err != nil && !errors.Is(err, zng.ErrUnset) {
				return nil, err
			}
			if g.valueCompare(keyVal, *g.maxSpillKey) >= 0 {
				break
			}
		}
		rec, err := g.nextResultFromSpills()
		if err != nil {
			return nil, err
		}
		if rec == nil {
			break
		}
		recs = append(recs, rec)
	}
	if len(recs) == 0 {
		return nil, nil
	}
	return zbuf.NewArray(recs), nil
}

func (g *GroupByAggregator) nextResultFromSpills() (*zng.Record, error) {
	// Consume all partial result records that have the same grouping keys.
	row := compile.NewRow(g.reducerDefs)
	var firstRec *zng.Record
	for {
		rec, err := g.runManager.Peek()
		if err != nil {
			return nil, err
		}
		if rec == nil {
			break
		}
		if firstRec == nil {
			firstRec = rec.Keep()
		} else if g.keysCompare(firstRec, rec) != 0 {
			break
		}
		if err := row.ConsumePart(rec); err != nil {
			return nil, err
		}
		if _, err := g.runManager.Read(); err != nil {
			return nil, err
		}
	}
	if firstRec == nil {
		return nil, nil
	}
	// Build the result record.
	g.builder.Reset()
	var types []zng.Type
	for _, res := range g.keyResolvers {
		keyVal := res(firstRec)
		types = append(types, keyVal.Type)
		g.builder.Append(keyVal.Bytes, keyVal.IsContainer())
	}
	zbytes, err := g.builder.Encode()
	if err != nil {
		return nil, err
	}
	cols := g.builder.TypedColumns(types)
	for i, red := range row.Reducers {
		var v zng.Value
		if g.emitPart {
			vv, err := red.(reducer.Decomposable).ResultPart(g.zctx)
			if err != nil {
				return nil, err
			}
			v = vv
		} else {
			v = red.Result()
		}
		cols = append(cols, zng.NewColumn(row.Defs[i].Target, v.Type))
		zbytes = v.Encode(zbytes)
	}
	typ, err := g.zctx.LookupTypeRecord(cols)
	if err != nil {
		return nil, err
	}
	return zng.NewRecord(typ, zbytes), nil
}

// readTable returns a slice of records from the in-memory groupby
// table. If flush is true, the entire table is returned. If flush is
// false and input is sorted only completed keys are returned.
// If decompose is true, it returns partial reducer results as
// returned by reducer.Decomposable.ResultPart(). It is an error to
// pass decompose=true if any reducer is non-decomposable.
func (g *GroupByAggregator) readTable(flush, decompose bool) (zbuf.Batch, error) {
	var recs []*zng.Record
	for k, row := range g.table {
		if !flush && g.valueCompare == nil {
			panic("internal bug: tried to fetch completed tuples on non-sorted input")
		}
		if !flush && g.valueCompare(*row.groupval, *g.maxTableKey) >= 0 {
			continue
		}
		var zv zcode.Bytes
		zv = append(zv, row.keyvals...)
		for _, red := range row.reducers.Reducers {
			var v zng.Value
			if decompose {
				var err error
				dec := red.(reducer.Decomposable)
				v, err = dec.ResultPart(g.zctx)
				if err != nil {
					return nil, err
				}
			} else {
				// a reducer value is never a container
				v = red.Result()
				if v.IsContainer() {
					panic("internal bug: reducer result cannot be a container!")
				}
			}
			zv = v.Encode(zv)
		}
		typ, err := g.lookupRowType(row, decompose)
		if err != nil {
			return nil, err
		}
		recs = append(recs, zng.NewRecord(typ, zv))
		delete(g.table, k)
	}
	if len(recs) == 0 {
		return nil, nil
	}
	return zbuf.NewArray(recs), nil
}

func (g *GroupByAggregator) lookupRowType(row *GroupByRow, decompose bool) (*zng.TypeRecord, error) {
	// This is only done once per row at output time so generally not a
	// bottleneck, but this could be optimized by keeping a cache of the
	// descriptor since it is rare for there to be multiple descriptors
	// or for it change from row to row.
	n := len(g.keys) + len(g.reducerDefs)
	cols := make([]zng.Column, 0, n)
	types := make([]zng.Type, len(row.keycols))

	for k, col := range row.keycols {
		types[k] = col.Type
	}
	cols = append(cols, g.builder.TypedColumns(types)...)
	for k, red := range row.reducers.Reducers {
		var z zng.Value
		if decompose {
			var err error
			z, err = red.(reducer.Decomposable).ResultPart(g.zctx)
			if err != nil {
				return nil, err
			}
		} else {
			z = red.Result()
		}
		cols = append(cols, zng.NewColumn(row.reducers.Defs[k].Target, z.Type))
	}
	// This could be more efficient but it's only done during group-by output...
	return g.zctx.LookupTypeRecord(cols)
}
