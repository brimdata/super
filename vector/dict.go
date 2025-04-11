package vector

import (
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Dict struct {
	loader DictLoader
	Any
	index  []byte
	counts []uint32
	nulls  bitvec.Bits
	length uint32
}

var _ Any = (*Dict)(nil)

func NewDict(vals Any, index []byte, counts []uint32, nulls bitvec.Bits) *Dict {
	return &Dict{Any: vals, index: index, counts: counts, nulls: nulls, length: uint32(len(index))}
}

func NewDictWithLoader(vals Any, loader DictLoader, length uint32) *Dict {
	return &Dict{Any: vals, loader: loader, length: length}
}

func (d *Dict) Len() uint32 {
	return d.length
}

func (d *Dict) Index() []byte {
	if d.index == nil {
		d.index, d.counts, d.nulls = d.loader.Load()
	}
	return d.index
}

func (d *Dict) Counts() []uint32 {
	if d.index == nil {
		d.index, d.counts, d.nulls = d.loader.Load()
	}
	return d.counts
}

func (d *Dict) Nulls() bitvec.Bits {
	if d.index == nil {
		d.index, d.counts, d.nulls = d.loader.Load()
	}
	return d.nulls
}

func (d *Dict) SetNulls(nulls bitvec.Bits) {
	d.nulls = nulls
}

func (d *Dict) Serialize(builder *zcode.Builder, slot uint32) {
	if d.Nulls().IsSet(slot) {
		builder.Append(nil)
	} else {
		d.Any.Serialize(builder, uint32(d.Index()[slot]))
	}
}

// RebuildDropIndex rebuilds the dictionary Index, Count and Nulls values with
// the passed in tags removed.
func (d *Dict) RebuildDropTags(tags ...uint32) ([]byte, []uint32, bitvec.Bits, []uint32) {
	m := make([]int, d.Any.Len())
	for _, i := range tags {
		m[i] = -1
	}
	var k = 0
	for i := range m {
		if m[i] != -1 {
			m[i] = k
			k++
		}
	}
	var nulls bitvec.Bits
	inNulls := d.Nulls()
	if !inNulls.IsZero() {
		nulls = bitvec.NewFalse(d.Len())
	}
	counts := make([]uint32, int(d.Any.Len())-len(tags))
	var index []byte
	var dropped []uint32
	for i, tag := range d.Index() {
		if inNulls.IsSet(uint32(i)) {
			nulls.Set(uint32(len(index)))
			index = append(index, 0)
			continue
		}
		k := m[tag]
		if k == -1 {
			dropped = append(dropped, uint32(i))
			continue
		}
		index = append(index, byte(k))
		counts[k]++
	}
	return index, counts, nulls, dropped
}
