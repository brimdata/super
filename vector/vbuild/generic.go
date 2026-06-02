package vbuild

import (
	"math"
	"slices"

	"github.com/brimdata/super/vector"
)

type genericBuilder[E comparable] struct {
	writer   genericWriter
	valuesOf func(vector.Any) []E
	build    func([]E) vector.Any
}

func (b *genericBuilder[E]) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	if b.writer == nil {
		b.writer = &genericConstWriter[E]{
			valuesOf: b.valuesOf,
			build:    b.build,
		}
	}
	b.writer = b.writer.Write(vec)
}

func (b *genericBuilder[E]) Build() vector.Any {
	if b.writer == nil {
		b.writer = &genericFlatWriter[E]{
			valuesOf: b.valuesOf,
			build:    b.build,
		}
	}
	return b.writer.Build()
}

type genericWriter interface {
	Write(vector.Any) genericWriter
	Build() vector.Any
}

type genericConstWriter[E comparable] struct {
	val      E
	len      uint32
	valuesOf func(vector.Any) []E
	build    func([]E) vector.Any
}

func (c *genericConstWriter[E]) Write(vec vector.Any) genericWriter {
	if vec.Len() == 0 {
		return c
	}
	if const_, ok := vec.(*vector.Const); ok {
		vals := c.valuesOf(const_.Any)
		if c.len == 0 {
			c.val = vals[0]
		}
		if c.val == vals[0] {
			c.len += const_.Len()
			return c
		}
	}
	w := genericWriter(&genericDictWriter[E]{
		dict:     make(map[E]byte),
		valuesOf: c.valuesOf,
		build:    c.build,
	})
	if c.len > 0 {
		w = w.Write(c.Build())
	}
	return w.Write(vec)
}

func (c *genericConstWriter[E]) Build() vector.Any {
	return vector.NewConst(c.build([]E{c.val}), c.len)
}

type genericDictWriter[E comparable] struct {
	dict     map[E]byte
	counts   []uint32
	index    []byte
	valuesOf func(vector.Any) []E
	build    func([]E) vector.Any
}

func (d *genericDictWriter[E]) Write(vec vector.Any) genericWriter {
	if vec.Len() == 0 {
		return d
	}
	switch vec := vec.(type) {
	case *vector.Const:
		vals := d.valuesOf(vec.Any)
		idx, ok := d.writeEntry(vals[0], vec.Len())
		if ok {
			d.index = append(d.index, slices.Repeat([]byte{idx}, int(vec.Len()))...)
			return d
		}
	case *vector.Dict:
		vals := d.valuesOf(vec.Any)
		remap := make([]byte, len(vals))
		var ok bool
		for i, val := range vals {
			if remap[i], ok = d.writeEntry(val, vec.Counts[i]); !ok {
				break
			}
		}
		if ok {
			for _, idx := range vec.Index {
				d.index = append(d.index, remap[idx])
			}
			return d
		}
	}
	w := genericWriter(&genericFlatWriter[E]{
		valuesOf: d.valuesOf,
		build:    d.build,
	})
	if len(d.index) > 0 {
		w = w.Write(d.Build())
	}
	return w.Write(vec)
}

func (d *genericDictWriter[E]) writeEntry(val E, count uint32) (byte, bool) {
	idx, ok := d.dict[val]
	if !ok {
		if len(d.counts) > math.MaxUint8 {
			return 0, false
		}
		idx = byte(len(d.counts))
		d.dict[val] = idx
		d.counts = append(d.counts, 0)
	}
	d.counts[idx] += count
	return idx, true
}

func (d *genericDictWriter[E]) Build() vector.Any {
	vals := make([]E, len(d.counts))
	for val, idx := range d.dict {
		vals[idx] = val
	}
	return vector.NewDict(d.build(vals), d.index, d.counts)
}

type genericFlatWriter[E comparable] struct {
	vals     []E
	valuesOf func(vector.Any) []E
	build    func([]E) vector.Any
}

func (f *genericFlatWriter[E]) Write(vec vector.Any) genericWriter {
	if vec.Len() == 0 {
		return f
	}
	switch vec := vec.(type) {
	case *vector.Const:
		vals := f.valuesOf(vec.Any)
		for range vec.Len() {
			f.vals = append(f.vals, vals[0])
		}
	case *vector.Dict:
		vals := f.valuesOf(vec.Any)
		for _, idx := range vec.Index {
			f.vals = append(f.vals, vals[idx])
		}
	case *vector.View:
		vals := f.valuesOf(vec.Any)
		for _, idx := range vec.Index {
			f.vals = append(f.vals, vals[idx])
		}
	default:
		f.vals = append(f.vals, f.valuesOf(vec)...)
	}
	return f
}

func (f *genericFlatWriter[E]) Build() vector.Any {
	return f.build(f.vals)
}
