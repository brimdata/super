package agg

import (
	"encoding/binary"

	"github.com/brimdata/super"
	samagg "github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

type distinct struct {
	fun  Func
	buf  []byte
	seen map[string]struct{}
}

func newDistinct(f Func) Func {
	return &distinct{fun: f, seen: map[string]struct{}{}}
}

func (d *distinct) Consume(vec vector.Any) {
	id := vec.Type().ID()
	var b scode.Builder
	for i := range vec.Len() {
		b.Truncate()
		vec.Serialize(&b, i)
		d.buf = binary.AppendVarint(d.buf[:0], int64(id))
		d.buf = append(d.buf, b.Bytes()...)
		if _, ok := d.seen[string(d.buf)]; ok {
			continue
		}
		d.seen[string(d.buf)] = struct{}{}
	}
}

func (d *distinct) ConsumeAsPartial(vec vector.Any) {
	if vec.Len() != 1 {
		panic("distinct: invalid partial")
	}
	if vector.NullsOf(vec).IsSet(0) {
		return
	}
	var slot uint32
	if view, ok := vec.(*vector.View); ok {
		vec = view.Any
		slot = view.Index[0]
	}
	array, ok := vec.(*vector.Array)
	if !ok {
		panic("distinct: invalid partial")
	}
	start, end, null := vector.ContainerOffset(array, slot)
	if null {
		return
	}
	values := array.Values
	if start > 0 || end < vec.Len() {
		index := make([]uint32, end-start)
		for i := range index {
			index[i] = start + uint32(i)
		}
		values = vector.Pick(values, index)
	}
	d.Consume(values)
}

func (d *distinct) Result(sctx *super.Context) super.Value {
	b := vector.NewDynamicBuilder()
	var count int
	for key := range d.seen {
		b.Write(samagg.NewValueFromDistinctKey(sctx, key))
		count++
		if count == 1024 {
			d.fun.Consume(b.Build())
			b = vector.NewDynamicBuilder()
			count = 0
		}
		delete(d.seen, key)
	}
	if count > 0 {
		d.fun.Consume(b.Build())
	}
	return d.fun.Result(sctx)
}

func (d *distinct) ResultAsPartial(sctx *super.Context) super.Value {
	return samagg.DistinctResultAsPartial(sctx, d.seen)
}
