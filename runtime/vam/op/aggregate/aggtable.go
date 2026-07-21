package aggregate

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/runtime/vam/expr/agg"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vbuild"
)

// XXX use super.Value for slow path stuff, e.g., when the grouping key is
// a complex type.  when we improve the super.Value impl this will get better.

// one aggTable per fixed set of types of aggs and keys.
type aggTable interface {
	update([]vector.Any, []vector.Any)
	materialize() vector.Any
}

type superTable struct {
	aggs        []*expr.Aggregator
	builder     *vector.RecordBuilder
	partialsIn  bool
	partialsOut bool
	table       map[string]int
	rows        []aggRow
	sctx        *super.Context

	// Reused across batches to avoid per-batch/per-row allocation in update.
	batchGroups map[int][]uint32
	kb          scode.Builder
}

var _ aggTable = (*superTable)(nil)

type aggRow struct {
	keys  []super.Value
	funcs []agg.Func
}

func (s *superTable) update(keys []vector.Any, args []vector.Any) {
	// Group this batch's slots by row id (index into s.rows) by probing the
	// global table directly.  This replaces the previous per-batch
	// map[string][]uint32 (a fresh map every batch) plus a per-row string
	// allocation: a key string is now allocated only when a genuinely new
	// group is created (O(distinct keys) rather than O(rows)), and the int
	// keyed grouping map is reused across batches.
	if s.batchGroups == nil {
		s.batchGroups = make(map[int][]uint32)
	}
	groups := s.batchGroups
	clear(groups)
	if len(keys) > 0 {
		b := &s.kb
		for slot := range keys[0].Len() {
			b.Truncate()
			for _, key := range keys {
				key.Serialize(b, slot)
			}
			body := []byte(b.Bytes())
			id, ok := s.table[string(body)] // no-alloc map lookup
			if !ok {
				id = len(s.rows)
				s.table[string(body)] = id // allocates the key string once
				s.rows = append(s.rows, s.newRowForSlot(keys, slot))
			}
			groups[id] = append(groups[id], slot)
		}
	} else {
		id, ok := s.table[""]
		if !ok {
			id = len(s.rows)
			s.table[""] = id
			s.rows = append(s.rows, s.newRowForSlot(keys, 0))
		}
		groups[id] = nil
	}
	single := len(groups) == 1
	for id, index := range groups {
		row := s.rows[id]
		for i, arg := range args {
			a := arg
			if !single {
				a = vector.Pick(a, index)
			}
			a, ok := removeQuiet(a)
			if !ok {
				continue
			}
			if s.partialsIn {
				row.funcs[i].ConsumeAsPartial(a)
			} else {
				row.funcs[i].Consume(a)
			}
		}
	}
}

// removeQuiet removes any error("quiet") values from vec.  It returns false if
// all values are error("quiet").
func removeQuiet(vec vector.Any) (vector.Any, bool) {
	if index, ok := notQuietIndex(vec); ok {
		if len(index) == 0 {
			// Every slot is error("quiet").
			return nil, false
		}
		return vector.Pick(vec, index), true
	}
	return vec, true
}

func (s *superTable) newRowForSlot(keys []vector.Any, slot uint32) aggRow {
	var row aggRow
	for _, agg := range s.aggs {
		row.funcs = append(row.funcs, agg.Pattern())
	}
	// Use a fresh builder here (not s.kb) because the serialized key bytes are
	// retained by the stored super.Value and must not be reused/truncated.
	var b scode.Builder
	for _, key := range keys {
		b.Reset()
		key.Serialize(&b, slot)
		row.keys = append(row.keys, super.NewValue(key.Type(), b.Bytes().Body()))
	}
	return row
}

func (s *superTable) materialize() vector.Any {
	if len(s.rows) == 0 {
		return vector.NewNull(0)
	}
	var vecs []vector.Any
	for i := range s.rows[0].keys {
		vecs = append(vecs, s.materializeKey(i))
	}
	for i := range s.rows[0].funcs {
		vecs = append(vecs, s.materializeAgg(i))
	}
	// Since aggs can return dynamic values need to do apply to create record.
	return vector.Apply(vector.ApplyNone, func(vecs ...vector.Any) vector.Any {
		return s.builder.New(vecs)
	}, vecs...)
}

func (s *superTable) materializeKey(i int) vector.Any {
	b := vector.NewValueBuilder(s.rows[0].keys[i].Type())
	for _, row := range s.rows {
		b.Write(row.keys[i].Bytes())
	}
	return b.Build(s.sctx)
}

func (s *superTable) materializeAgg(i int) vector.Any {
	b := vbuild.NewDynamicBuilder()
	for _, row := range s.rows {
		if s.partialsOut {
			b.Write(row.funcs[i].ResultAsPartial(s.sctx))
		} else {
			b.Write(row.funcs[i].Result(s.sctx))
		}
	}
	return b.Build()
}

type countByString struct {
	typ        super.Type
	table      map[string]int64
	builder    *vector.RecordBuilder
	partialsIn bool
}

func newCountByString(typ super.Type, b *vector.RecordBuilder, partialsIn bool) aggTable {
	return &countByString{
		typ:        typ,
		builder:    b,
		table:      make(map[string]int64),
		partialsIn: partialsIn,
	}
}

func (c *countByString) update(keys, vals []vector.Any) {
	if c.partialsIn {
		c.updatePartial(keys[0], vals[0])
		return
	}
	switch val := vector.Under(keys[0]).(type) {
	case *vector.String:
		c.count(val)
	case *vector.Dict:
		c.countDict(val.Any.(*vector.String), val.Counts)
	case *vector.Const:
		c.countFixed(val)
	case *vector.View:
		c.countView(val)
	default:
		panic(fmt.Sprintf("UNKNOWN %T", val))
	}
}

func (c *countByString) updatePartial(keyvec, valvec vector.Any) {
	key, ok1 := vector.Under(keyvec).(*vector.String)
	val, ok2 := valvec.(*vector.Int)
	if !ok1 || !ok2 {
		panic("count by string: invalid partials in")
	}
	for i := range key.Len() {
		c.table[key.Value(i)] += val.Values[i]
	}
}

func (c *countByString) count(vec *vector.String) {
	offs, bytes := vec.Table().Slices()
	for k := range vec.Len() {
		c.table[string(bytes[offs[k]:offs[k+1]])]++
	}
}

func (c *countByString) countDict(vec *vector.String, counts []uint32) {
	offs, bytes := vec.Table().Slices()
	for k := range vec.Len() {
		if counts[k] > 0 {
			c.table[string(bytes[offs[k]:offs[k+1]])] += int64(counts[k])
		}
	}
}

func (c *countByString) countFixed(vec *vector.Const) {
	c.table[vector.StringValue(vec, 0)] += int64(vec.Len())
}

func (c *countByString) countView(vec *vector.View) {
	strVec := vec.Any.(*vector.String)
	for _, slot := range vec.Index {
		c.table[strVec.Value(slot)]++
	}
}

func (c *countByString) materialize() vector.Any {
	length := len(c.table)
	counts := make([]int64, length)
	var bytes []byte
	offs := make([]uint32, length+1)
	var k int
	for key, count := range c.table {
		offs[k] = uint32(len(bytes))
		bytes = append(bytes, key...)
		counts[k] = count
		k++
	}
	offs[k] = uint32(len(bytes))
	keyVec := vector.Any(vector.NewString(vector.NewBytesTable(offs, bytes)))
	if n, ok := c.typ.(*super.TypeNamed); ok {
		keyVec = vector.NewNamed(n, keyVec)
	}
	countVec := vector.NewInt(super.TypeInt64, counts)
	return c.builder.New([]vector.Any{keyVec, countVec})
}
