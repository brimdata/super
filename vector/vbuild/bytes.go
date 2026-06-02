package vbuild

import (
	"bytes"
	"math"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type bytesBuilder struct {
	writer genericWriter
}

func newBytesBuilder(typ super.Type) Builder {
	return &bytesBuilder{
		writer: &bytesConstWriter{typ: typ},
	}
}

func (s *bytesBuilder) Write(vec vector.Any) {
	s.writer = s.writer.Write(vec)
}

func (s *bytesBuilder) Build() vector.Any {
	return s.writer.Build()
}

type bytesConstWriter struct {
	typ super.Type
	val []byte
	len uint32
}

func (b *bytesConstWriter) Write(vec vector.Any) genericWriter {
	if vec.Len() == 0 {
		return b
	}
	if c, ok := vec.(*vector.Const); ok {
		v := bytesTableOf(c.Any).Bytes(0)
		if b.val == nil {
			b.val = slices.Clone(v)
		}
		if bytes.Equal(b.val, v) {
			b.len += c.Len()
			return b
		}
	}
	writer := genericWriter(&bytesDictWriter{
		typ:  b.typ,
		dict: make(map[string]byte),
	})
	if b.len > 0 {
		writer = writer.Write(b.Build())
	}
	return writer.Write(vec)
}

func (b *bytesConstWriter) Build() vector.Any {
	table := vector.NewBytesTableEmpty(0)
	table.Append(b.val)
	return vector.NewConst(newBytesOrStringVector(b.typ, table), b.len)
}

type bytesDictWriter struct {
	typ    super.Type
	dict   map[string]byte
	counts []uint32
	index  []byte
}

func (b *bytesDictWriter) Write(vec vector.Any) genericWriter {
	if vec.Len() == 0 {
		return b
	}
	switch vec := vec.(type) {
	case *vector.Const:
		t := bytesTableOf(vec.Any)
		slot, ok := b.writeEntry(t.Bytes(0), vec.Len())
		if ok {
			b.index = slices.Grow(b.index, int(vec.Len()))
			for range vec.Len() {
				b.index = append(b.index, slot)
			}
			return b
		}
	case *vector.Dict:
		t := bytesTableOf(vec.Any)
		remap := make([]byte, t.Len())
		var ok bool
		for i := range t.Len() {
			if remap[i], ok = b.writeEntry(t.Bytes(i), vec.Counts[i]); !ok {
				break
			}
		}
		if ok {
			for _, idx := range vec.Index {
				b.index = append(b.index, remap[idx])
			}
			return b
		}
	}
	writer := genericWriter(&bytesFlatWriter{
		typ:   b.typ,
		table: vector.NewBytesTableEmpty(0),
	})
	if len(b.index) > 0 {
		writer = writer.Write(b.Build())
	}
	return writer.Write(vec)
}

func (b *bytesDictWriter) writeEntry(val []byte, count uint32) (byte, bool) {
	slot, ok := b.dict[string(val)]
	if !ok {
		if len(b.counts) > math.MaxUint8 {
			return 0, false
		}
		slot = byte(len(b.counts))
		b.dict[string(val)] = slot
		b.counts = append(b.counts, 0)
	}
	b.counts[slot] += count
	return slot, true
}

func (b *bytesDictWriter) Build() vector.Any {
	vals := make([][]byte, len(b.counts))
	for s, idx := range b.dict {
		vals[idx] = []byte(s)
	}
	table := vector.NewBytesTableEmpty(0)
	for _, s := range vals {
		table.Append(s)
	}
	return vector.NewDict(newBytesOrStringVector(b.typ, table), b.index, b.counts)
}

type bytesFlatWriter struct {
	typ   super.Type
	table vector.BytesTable
}

func (b *bytesFlatWriter) Write(vec vector.Any) genericWriter {
	if vec.Len() == 0 {
		return b
	}
	switch vec := vec.(type) {
	case *vector.View:
		table := bytesTableOf(vec.Any)
		for _, slot := range vec.Index {
			b.table.Append(table.Bytes(slot))
		}
	case *vector.Const:
		bytes := bytesTableOf(vec.Any).Bytes(0)
		for range vec.Len() {
			b.table.Append(bytes)
		}
	case *vector.Dict:
		table := bytesTableOf(vec.Any)
		for _, slot := range vec.Index {
			b.table.Append(table.Bytes(uint32(slot)))
		}
	case *vector.String, *vector.Bytes:
		table := bytesTableOf(vec)
		for i := range vec.Len() {
			b.table.Append(table.Bytes(i))
		}
	default:
		panic(vec)
	}
	return b
}

func (b *bytesFlatWriter) Build() vector.Any {
	return newBytesOrStringVector(b.typ, b.table)
}

func bytesTableOf(vec vector.Any) vector.BytesTable {
	switch vec := vec.(type) {
	case *vector.String:
		return vec.Table()
	case *vector.Bytes:
		return vec.Table()
	case *vector.Empty:
		return vector.BytesTable{}
	default:
		panic(vec)
	}
}

func newBytesOrStringVector(typ super.Type, table vector.BytesTable) vector.Any {
	switch typ.ID() {
	case super.IDString:
		return vector.NewString(table)
	case super.IDBytes:
		return vector.NewBytes(table)
	default:
		panic(typ)
	}
}
