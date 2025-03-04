package summarize

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/runtime/vam/expr/agg"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

// XXX use super.Value for slow path stuff, e.g., when the group-by key is
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
	zctx        *super.Context
}

var _ aggTable = (*superTable)(nil)

type aggRow struct {
	keys  []super.Value
	funcs []agg.Func
}

func (s *superTable) update(keys []vector.Any, args []vector.Any) {
	m := make(map[string][]uint32)
	if len(keys) > 0 {
		var b zcode.Builder
		for slot := range keys[0].Len() {
			b.Truncate()
			for _, key := range keys {
				key.Serialize(&b, slot)
			}
			keyStr := string(b.Bytes())
			m[keyStr] = append(m[keyStr], slot)
		}
	} else {
		m[""] = nil
	}
	for rowKey, index := range m {
		id, ok := s.table[rowKey]
		if !ok {
			id = len(s.rows)
			s.table[rowKey] = id
			s.rows = append(s.rows, s.newRow(keys, index))
		}
		row := s.rows[id]
		for i, arg := range args {
			if len(m) > 1 {
				arg = vector.NewView(arg, index)
			}
			if s.partialsIn {
				row.funcs[i].ConsumeAsPartial(arg)
			} else {
				row.funcs[i].Consume(arg)
			}
		}
	}
}

func (s *superTable) newRow(keys []vector.Any, index []uint32) aggRow {
	var row aggRow
	for _, agg := range s.aggs {
		row.funcs = append(row.funcs, agg.Pattern())
	}
	var b zcode.Builder
	for _, key := range keys {
		b.Reset()
		key.Serialize(&b, index[0])
		row.keys = append(row.keys, super.NewValue(key.Type(), b.Bytes().Body()))
	}
	return row
}

func (s *superTable) materialize() vector.Any {
	if len(s.rows) == 0 {
		return vector.NewConst(super.Null, 0, nil)
	}
	var vecs []vector.Any
	for i := range s.rows[0].keys {
		vecs = append(vecs, s.materializeKey(i))
	}
	for i := range s.rows[0].funcs {
		vecs = append(vecs, s.materializeAgg(i))
	}
	// Since aggs can return dynamic values need to do apply to create record.
	return vector.Apply(false, func(vecs ...vector.Any) vector.Any {
		return s.builder.New(vecs, nil)
	}, vecs...)
}

func (s *superTable) materializeKey(i int) vector.Any {
	b := vector.NewBuilder(s.rows[0].keys[i].Type())
	for _, row := range s.rows {
		b.Write(row.keys[i].Bytes())
	}
	return b.Build()
}

func (s *superTable) materializeAgg(i int) vector.Any {
	b := vector.NewDynamicBuilder()
	for _, row := range s.rows {
		if s.partialsOut {
			b.Write(row.funcs[i].ResultAsPartial(s.zctx))
		} else {
			b.Write(row.funcs[i].Result(s.zctx))
		}
	}
	return b.Build()
}

type countByString struct {
	nulls      uint64
	table      map[string]uint64
	builder    *vector.RecordBuilder
	partialsIn bool
}

func newCountByString(b *vector.RecordBuilder, partialsIn bool) aggTable {
	return &countByString{
		builder:    b,
		table:      make(map[string]uint64),
		partialsIn: partialsIn,
	}
}

func (c *countByString) update(keys, vals []vector.Any) {
	if c.partialsIn {
		c.updatePartial(keys[0], vals[0])
		return
	}
	switch val := keys[0].(type) {
	case *vector.String:
		c.count(val)
	case *vector.Dict:
		c.countDict(val.Any.(*vector.String), val.Counts, val.Nulls)
	case *vector.Const:
		c.countFixed(val)
	case *vector.View:
		c.countView(val)
	default:
		panic(fmt.Sprintf("UNKNOWN %T", val))
	}
}

func (c *countByString) updatePartial(keyvec, valvec vector.Any) {
	key, ok1 := keyvec.(*vector.String)
	val, ok2 := valvec.(*vector.Uint)
	if !ok1 || !ok2 {
		panic("count by string: invalid partials in")
	}
	if key.Nulls != nil {
		for i := range key.Len() {
			if key.Nulls.Value(i) {
				c.nulls += val.Values[i]
			} else {
				c.table[key.Value(i)] += val.Values[i]
			}
		}
	} else {
		for i := range key.Len() {
			c.table[key.Value(i)] += val.Values[i]
		}
	}
}

func (c *countByString) count(vec *vector.String) {
	offs := vec.Offsets
	bytes := vec.Bytes
	if vec.Nulls == nil {
		for k := range vec.Len() {
			c.table[string(bytes[offs[k]:offs[k+1]])]++
		}
	} else {
		for k := range vec.Len() {
			if vec.Nulls.Value(k) {
				c.nulls++
			} else {
				c.table[string(bytes[offs[k]:offs[k+1]])]++
			}
		}
	}
}

func (c *countByString) countDict(vec *vector.String, counts []uint32, nulls *vector.Bool) {
	offs := vec.Offsets
	bytes := vec.Bytes
	for k := range vec.Len() {
		if counts[k] > 0 {
			c.table[string(bytes[offs[k]:offs[k+1]])] += uint64(counts[k])
		}
	}
	if nulls != nil {
		for k := range nulls.Len() {
			if nulls.Value(k) {
				c.nulls++
			}
		}
	}
}

func (c *countByString) countFixed(vec *vector.Const) {
	val := vec.Value()
	switch val.Type().ID() {
	case super.IDString:
		var nullCnt uint64
		if vec.Nulls != nil {
			for k := range vec.Len() {
				if vec.Nulls.Value(k) {
					nullCnt++
				}
			}
			c.nulls += nullCnt
		}
		c.table[super.DecodeString(val.Bytes())] += uint64(vec.Len()) - nullCnt
	case super.IDNull:
		c.nulls += uint64(vec.Len())
	}
}

func (c *countByString) countView(vec *vector.View) {
	strVec := vec.Any.(*vector.String)
	if strVec.Nulls == nil {
		for _, slot := range vec.Index {
			c.table[strVec.Value(slot)]++
		}
	} else {
		for _, slot := range vec.Index {
			if strVec.Nulls.Value(slot) {
				c.nulls++
			} else {
				c.table[strVec.Value(slot)]++
			}
		}
	}
}

func (c *countByString) materialize() vector.Any {
	length := len(c.table)
	counts := make([]uint64, length)
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
	var nulls *vector.Bool
	if c.nulls > 0 {
		length++
		counts = append(counts, c.nulls)
		offs = append(offs, uint32(len(bytes)))
		nulls = vector.NewBoolEmpty(uint32(length), nil)
		nulls.Set(uint32(length - 1))
	}
	keyVec := vector.NewString(offs, bytes, nulls)
	countVec := vector.NewUint(super.TypeUint64, counts, nil)
	return c.builder.New([]vector.Any{keyVec, countVec}, nil)
}
