package op

import (
	"fmt"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

//XXX need to make sure vam operator objects are returned to GC as they are finished

type CountByString struct {
	parent vector.Puller
	zctx   *super.Context
	field  expr.Evaluator
	name   string
	table  countByString
	done   bool
}

func NewCountByString(zctx *super.Context, parent vector.Puller, name string) *CountByString {
	return &CountByString{
		parent: parent,
		zctx:   zctx,
		field:  expr.NewDotExpr(zctx, &expr.This{}, name),
		name:   name,
		table:  countByString{table: make(map[string]uint64)}, //XXX
	}
}

func (c *CountByString) Pull(done bool) (vector.Any, error) {
	if done {
		_, err := c.parent.Pull(done)
		return nil, err
	}
	if c.done {
		return nil, nil
	}
	for {
		//XXX check context Done
		vec, err := c.parent.Pull(false)
		if err != nil {
			return nil, err
		}
		if vec == nil {
			c.done = true
			return c.table.materialize(c.zctx, c.name), nil
		}
		c.update(vec)
	}
}

func (c *CountByString) update(val vector.Any) {
	if val, ok := val.(*vector.Dynamic); ok {
		for _, val := range val.Values {
			c.update(val)
		}
		return
	}
	switch val := c.field.Eval(val).(type) {
	case *vector.String:
		c.table.count(val)
	case *vector.Dict:
		c.table.countDict(val.Any.(*vector.String), val.Counts)
	case *vector.Const:
		c.table.countFixed(val)
	default:
		panic(fmt.Sprintf("UNKNOWN %T", val))
	}
}

type countByString struct {
	nulls uint64
	table map[string]uint64
}

func (c *countByString) count(vec *vector.String) {
	offs := vec.Offsets
	bytes := vec.Bytes
	n := len(offs) - 1
	for k := 0; k < n; k++ {
		c.table[string(bytes[offs[k]:offs[k+1]])]++
	}
}

func (c *countByString) countDict(vec *vector.String, counts []uint32) {
	offs := vec.Offsets
	bytes := vec.Bytes
	n := len(offs) - 1
	for k := 0; k < n; k++ {
		c.table[string(bytes[offs[k]:offs[k+1]])] = uint64(counts[k])
	}
}

func (c *countByString) countFixed(vec *vector.Const) {
	//XXX
	val := vec.Value()
	switch val.Type().ID() {
	case super.IDString:
		c.table[super.DecodeString(val.Bytes())] += uint64(vec.Length())
	case super.IDNull:
		c.nulls += uint64(vec.Length())
	}
}

func (c *countByString) materialize(zctx *super.Context, name string) *vector.Record {
	typ := zctx.MustLookupTypeRecord([]super.Field{
		{Type: super.TypeString, Name: name},
		{Type: super.TypeUint64, Name: "count"},
	})
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
	// XXX change nulls to string null... this will be fixed in
	// prod-quality summarize op
	var nulls *vector.Bool
	if c.nulls > 0 {
		length++
		counts = slices.Grow(counts, length)[0:length]
		offs = slices.Grow(offs, length+1)[0 : length+1]
		counts[k] = c.nulls
		k++
		offs[k] = uint32(len(bytes))
		nulls = vector.NewBoolEmpty(uint32(k), nil)
		nulls.Set(uint32(k - 1))
	}
	keyVec := vector.NewString(offs, bytes, nulls)
	countVec := vector.NewUint(super.TypeUint64, counts, nil)
	return vector.NewRecord(typ, []vector.Any{keyVec, countVec}, uint32(length), nil)
}

type Sum struct {
	parent vector.Puller
	zctx   *super.Context
	field  expr.Evaluator
	name   string
	sum    int64
	done   bool
}

func NewSum(zctx *super.Context, parent vector.Puller, name string) *Sum {
	return &Sum{
		parent: parent,
		zctx:   zctx,
		field:  expr.NewDotExpr(zctx, &expr.This{}, name),
		name:   name,
	}
}

func (c *Sum) Pull(done bool) (vector.Any, error) {
	if done {
		_, err := c.parent.Pull(done)
		return nil, err
	}
	if c.done {
		return nil, nil
	}
	for {
		//XXX check context Done
		// XXX PullVec returns a single vector and enumerates through the
		// different underlying types that match a particular projection
		vec, err := c.parent.Pull(false)
		if err != nil {
			return nil, err
		}
		if vec == nil {
			c.done = true
			return c.materialize(c.zctx, c.name), nil
		}
		c.update(vec)
	}
}

func (c *Sum) update(vec vector.Any) {
	if vec, ok := vec.(*vector.Dynamic); ok {
		for _, vec := range vec.Values {
			c.update(vec)
		}
		return
	}
	switch vec := c.field.Eval(vec).(type) {
	case *vector.Int:
		for _, x := range vec.Values {
			c.sum += x
		}
	case *vector.Uint:
		for _, x := range vec.Values {
			c.sum += int64(x)
		}
	case *vector.Dict:
		switch number := vec.Any.(type) {
		case *vector.Int:
			for k, val := range number.Values {
				c.sum += val * int64(vec.Counts[k])
			}
		case *vector.Uint:
			for k, val := range number.Values {
				c.sum += int64(val) * int64(vec.Counts[k])
			}
		}
	}
}

func (c *Sum) materialize(zctx *super.Context, name string) *vector.Record {
	typ := zctx.MustLookupTypeRecord([]super.Field{
		{Type: super.TypeInt64, Name: "sum"},
	})
	return vector.NewRecord(typ, []vector.Any{vector.NewInt(super.TypeInt64, []int64{c.sum}, nil)}, 1, nil)
}
