package agg

import (
	"encoding/binary"
	"fmt"

	"github.com/brimdata/super"
)

type distinct struct {
	fun      Function
	buf      []byte
	seen     map[string]struct{}
	size     int
	partials [][]byte
}

func newDistinct(f Function) Function {
	return &distinct{fun: f, seen: map[string]struct{}{}}
}

func (d *distinct) Consume(val super.Value) {
	d.buf = binary.AppendVarint(d.buf[:0], int64(val.Type().ID()))
	d.buf = append(d.buf, val.Bytes()...)
	if _, ok := d.seen[string(d.buf)]; ok {
		return
	}
	d.seen[string(d.buf)] = struct{}{}
	d.size += 1 + len(d.buf)
}

func (d *distinct) ConsumeAsPartial(val super.Value) {
	if val.Type() != super.TypeBytes {
		panic("distinct: invalid partial")
	}
	d.partials = append(d.partials, val.Bytes())
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
		d.fun.Consume(super.NewValue(typ, bytes))
		delete(d.seen, key)
	}
	return d.fun.Result(zctx)
}

func (d *distinct) ResultAsPartial(zctx *super.Context) super.Value {
	buf := make([]byte, 0, d.size)
	for key := range d.seen {
		buf = binary.AppendUvarint(buf, uint64(len(key)))
		buf = append(buf, key...)
	}
	return super.NewBytes(buf)
}
