package vector

import (
	"sync/atomic"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

// Dynamic is an ordered sequence of values taken from one or more
// hetereogenously-typed vectors.
type Dynamic struct {
	Tags   []uint32
	Values []Any
	tagMap atomic.Pointer[TagMap]
}

var _ Any = (*Dynamic)(nil)

func NewDynamic(tags []uint32, values []Any) *Dynamic {
	return &Dynamic{Tags: tags, Values: values}
}

func (*Dynamic) Type() super.Type {
	panic("can't call Type() on a vector.Dynamic")
}

func (d *Dynamic) TypeOf(slot uint32) super.Type {
	vals := d.Values[d.Tags[slot]]
	if v2, ok := vals.(*Dynamic); ok {
		return v2.TypeOf(d.TagMap().Forward[slot])
	}
	return vals.Type()
}

func (d *Dynamic) Len() uint32 {
	if d.Tags != nil {
		return uint32(len(d.Tags))
	}
	var length uint32
	for _, val := range d.Values {
		length += val.Len()
	}
	return length
}

func (d *Dynamic) Serialize(b *zcode.Builder, slot uint32) {
	d.Values[d.Tags[slot]].Serialize(b, d.TagMap().Forward[slot])
}

func (d *Dynamic) TagMap() *TagMap {
	if t := d.tagMap.Load(); t != nil {
		return t
	}
	if t := NewTagMap(d.Tags, d.Values); d.tagMap.CompareAndSwap(nil, t) {
		return t
	}
	return d.tagMap.Load()
}
