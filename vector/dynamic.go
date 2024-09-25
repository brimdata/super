package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

// Dynamic is an ordered sequence of values taken from one or more
// hetereogenously-typed vectors.
type Dynamic struct {
	Tags   []uint32
	Values []Any
	TagMap *TagMap
}

var _ Any = (*Dynamic)(nil)

func NewDynamic(tags []uint32, values []Any) *Dynamic {
	return &Dynamic{Tags: tags, Values: values, TagMap: NewTagMap(tags, values)}
}

func (*Dynamic) Type() zed.Type {
	panic("can't call Type() on a vector.Dynamic")
}

func (d *Dynamic) TypeOf(slot uint32) zed.Type {
	vals := d.Values[d.Tags[slot]]
	if v2, ok := vals.(*Dynamic); ok {
		return v2.TypeOf(d.TagMap.Forward[slot])
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
	d.Values[d.Tags[slot]].Serialize(b, d.TagMap.Forward[slot])
}

func (d *Dynamic) AppendKey(b []byte, slot uint32) []byte {
	return d.Values[d.Tags[slot]].AppendKey(b, d.TagMap.Forward[slot])
}
