package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type OptionEncoder struct {
	values Encoder
	tags   *Uint32Encoder
	count  uint32
	typ    super.Type
}

var _ Encoder = (*UnionEncoder)(nil)

func NewOptionEncoder(cctx *Context, vec *vector.Option) *OptionEncoder {
	var tags []uint32
	values := vec.Any
	if d, ok := values.(*vector.Dynamic); ok {
		//XXX should use RLE
		tags = d.Tags
		values = d.Values[super.OptionSomeTag]
	}
	return &OptionEncoder{
		values: NewEncoder(cctx, values),
		tags:   NewUint32Encoder(tags),
		count:  vec.Len(),
		typ:    vec.Type(),
	}
}

func (o *OptionEncoder) Emit(w io.Writer) error {
	if err := o.tags.Emit(w); err != nil {
		return err
	}
	return o.values.Emit(w)
}

func (o *OptionEncoder) Encode(group *errgroup.Group) {
	o.tags.Encode(group)
	o.values.Encode(group)
}

func (o *OptionEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, tags := o.tags.Segment(off)
	off, id := o.values.Metadata(cctx, off)
	return off, cctx.enter(&Option{
		Tags:   tags,
		Values: id,
		Length: o.count,
		Type:   o.typ,
	})
}
