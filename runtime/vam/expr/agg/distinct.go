package agg

import (
	"encoding/binary"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type distinct struct {
	Func
	buf      []byte
	seen     map[string]struct{}
	size     int
	partials [][]byte
}

func newDistinct(f Func) Func {
	return &distinct{Func: f, seen: map[string]struct{}{}}
}

func (d *distinct) Consume(vec vector.Any) {
	id := vec.Type().ID()
	var b zcode.Builder
	for i := range vec.Len() {
		b.Truncate()
		vec.Serialize(&b, i)
		d.buf = binary.AppendVarint(d.buf[:0], int64(id))
		d.buf = append(d.buf, b.Bytes().Body()...)
		if _, ok := d.seen[string(d.buf)]; ok {
			continue
		}
		d.seen[string(d.buf)] = struct{}{}
		d.size += 1 + len(d.buf)
	}
}

func (d *distinct) ConsumeAsPartial(vec vector.Any) {
	if vec.Type() != super.TypeBytes || vec.Len() != 1 {
		panic("distinct: invalid partial")
	}
	d.partials = append(d.partials, vec.(*vector.Bytes).Value(0))
}

func (d *distinct) Result(zctx *super.Context) super.Value {
	for i, partial := range d.partials {
		for len(partial) > 0 {
			length, n := binary.Uvarint(partial)
			if n <= 0 {
				panic(fmt.Sprintf("bad varint: %d", n))
			}
			partial = partial[n:]
			d.seen[string(partial[:length])] = struct{}{}
			partial = partial[length:]
		}
		d.partials[i] = nil
	}
	b := vector.NewDynamicBuilder()
	var count int
	for key := range d.seen {
		bytes := []byte(key)
		id, n := binary.Varint(bytes)
		if n <= 0 {
			panic(fmt.Sprintf("bad varint: %d", n))
		}
		bytes = bytes[n:]
		typ, err := zctx.LookupType(int(id))
		if err != nil {
			panic(err)
		}
		b.Write(super.NewValue(typ, bytes))
		count++
		if count == 1024 {
			d.Func.Consume(b.Build())
			b = vector.NewDynamicBuilder()
			count = 0
		}
		delete(d.seen, key)
	}
	if count > 0 {
		d.Func.Consume(b.Build())
	}
	return d.Func.Result(zctx)
}

func (d *distinct) ResultAsPartial(*super.Context) super.Value {
	buf := make([]byte, 0, d.size)
	for key := range d.seen {
		buf = binary.AppendUvarint(buf, uint64(len(key)))
		buf = append(buf, key...)
	}
	return super.NewBytes(buf)
}
