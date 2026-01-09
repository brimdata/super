package agg

import (
	"encoding/binary"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

type distinct struct {
	fun  Function
	buf  []byte
	seen map[string]struct{}
}

func newDistinct(f Function) Function {
	return &distinct{fun: f, seen: map[string]struct{}{}}
}

func (d *distinct) Consume(val super.Value) {
	d.buf = binary.AppendVarint(d.buf[:0], int64(val.Type().ID()))
	d.buf = scode.Append(d.buf, val.Bytes())
	if _, ok := d.seen[string(d.buf)]; ok {
		return
	}
	d.seen[string(d.buf)] = struct{}{}
}

func (d *distinct) ConsumeAsPartial(val super.Value) {
	if val.IsNull() {
		return
	}
	arrayType, ok := val.Type().(*super.TypeArray)
	if !ok {
		panic(fmt.Errorf("distinct partial is not an array: %s", sup.FormatValue(val)))
	}
	typ := arrayType.Type
	for it := val.Iter(); !it.Done(); {
		d.Consume(super.NewValue(typ, it.Next()))
	}
}

func (d *distinct) Result(sctx *super.Context) super.Value {
	for key := range d.seen {
		d.fun.Consume(NewValueFromDistinctKey(sctx, key))
		delete(d.seen, key)
	}
	return d.fun.Result(sctx)
}

func (d *distinct) ResultAsPartial(sctx *super.Context) super.Value {
	return DistinctResultAsPartial(sctx, d.seen)
}

func DistinctResultAsPartial(sctx *super.Context, seen map[string]struct{}) super.Value {
	vals := make([]super.Value, 0, len(seen))
	for key := range seen {
		vals = append(vals, NewValueFromDistinctKey(sctx, key))
		delete(seen, key)
	}
	return newArray(sctx, vals)
}

func NewValueFromDistinctKey(sctx *super.Context, key string) super.Value {
	bytes := []byte(key)
	id, n := binary.Varint(bytes)
	if n <= 0 {
		panic(fmt.Sprintf("bad varint: %d", n))
	}
	bytes = bytes[n:]
	typ, err := sctx.LookupType(int(id))
	if err != nil {
		panic(err)
	}
	return super.NewValue(typ, scode.Bytes(bytes).Body())
}
