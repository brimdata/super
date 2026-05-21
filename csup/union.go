package csup

import (
	"io"

	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type UnionEncoder struct {
	values    []Encoder
	rleOrTags *Uint32Encoder
	count     uint32
}

var _ Encoder = (*UnionEncoder)(nil)

func NewUnionEncoder(cctx *Context, vec *vector.Union) *UnionEncoder {
	var values []Encoder
	for _, val := range vec.Values() {
		values = append(values, NewEncoder(cctx, val))
	}
	var rleOrTags []uint32
	if len(vec.Typ.Types) == 2 {
		rleOrTags = vec.TagsRLE()
	} else {
		rleOrTags = vec.Tags()
	}
	return &UnionEncoder{
		values:    values,
		rleOrTags: NewUint32Encoder(rleOrTags),
		count:     vec.Len(),
	}
}

func (u *UnionEncoder) Emit(w io.Writer) error {
	if err := u.rleOrTags.Emit(w); err != nil {
		return err
	}
	for _, value := range u.values {
		if err := value.Emit(w); err != nil {
			return err
		}
	}
	return nil
}

func (u *UnionEncoder) Encode(group *errgroup.Group) {
	u.rleOrTags.Encode(group)
	for _, value := range u.values {
		value.Encode(group)
	}
}

func (u *UnionEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, tags := u.rleOrTags.Segment(off)
	values := make([]ID, 0, len(u.values))
	for _, val := range u.values {
		var id ID
		off, id = val.Metadata(cctx, off)
		values = append(values, id)
	}
	return off, cctx.enter(&Union{
		Tags:   tags,
		Values: values,
		Length: u.count,
	})
}
