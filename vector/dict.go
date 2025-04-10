package vector

import (
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Dict struct {
	Any
	Index  []byte
	Counts []uint32
	Nulls  bitvec.Bits
}

var _ Any = (*Dict)(nil)

func NewDict(vals Any, index []byte, counts []uint32, nulls bitvec.Bits) *Dict {
	return &Dict{vals, index, counts, nulls}
}

func (d *Dict) Len() uint32 {
	return uint32(len(d.Index))
}

func (d *Dict) Serialize(builder *zcode.Builder, slot uint32) {
	if d.Nulls.IsSet(slot) {
		builder.Append(nil)
	} else {
		d.Any.Serialize(builder, uint32(d.Index[slot]))
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
	if !d.Nulls.IsZero() {
		nulls = bitvec.NewFalse(d.Len())
	}
	counts := make([]uint32, int(d.Any.Len())-len(tags))
	var index []byte
	var dropped []uint32
	for i, tag := range d.Index {
		if d.Nulls.IsSet(uint32(i)) {
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
