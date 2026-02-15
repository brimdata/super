package vector

import (
	"github.com/brimdata/super/scode"
)

type Dict struct {
	Any
	Index  []byte
	Counts []uint32
}

func NewDict(vals Any, index []byte, counts []uint32) *Dict {
	return &Dict{vals, index, counts}
}

func (d *Dict) Len() uint32 {
	return uint32(len(d.Index))
}

func (d *Dict) Serialize(builder *scode.Builder, slot uint32) {
	d.Any.Serialize(builder, uint32(d.Index[slot]))
}

// RebuildDropIndex rebuilds the dictionary Index and Count values with tags removed.
func (d *Dict) RebuildDropTags(tags ...uint32) ([]byte, []uint32, []uint32) {
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
	counts := make([]uint32, int(d.Any.Len())-len(tags))
	var index []byte
	var dropped []uint32
	for i, tag := range d.Index {
		k := m[tag]
		if k == -1 {
			dropped = append(dropped, uint32(i))
			continue
		}
		index = append(index, byte(k))
		counts[k]++
	}
	return index, counts, dropped
}
