package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type UnionEncoder struct {
	typ    *super.TypeUnion
	values []Encoder
	tags   Uint32Encoder
	count  uint32
}

var _ Encoder = (*UnionEncoder)(nil)

func NewUnionEncoder(cctx *Context, typ *super.TypeUnion) *UnionEncoder {
	var values []Encoder
	for _, typ := range typ.Types {
		values = append(values, NewEncoder(cctx, typ))
	}
	return &UnionEncoder{
		typ:    typ,
		values: values,
	}
}

func (u *UnionEncoder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	union := vec.(*vector.Union)
	u.count += vec.Len()
	// Union vectors do not require that the values slice has
	// alignment with the types in the union type.  Thus, we can
	// have vectors land here that have different orderings for
	// the same union type.  We could optimize this by adopting the
	// order of the first vector and recomputing the tags for each
	// subsequent incoming vector so that we don't have to rewrite
	// the tags of the first vector, but for now, we just map
	// everything to canonical order of the union types.
	vecs, tags := u.reorder(u.typ, union)
	u.tags.Append(tags)
	for k, vec := range vecs {
		u.values[k].Write(vec)
	}
}

func (u *UnionEncoder) reorder(typ *super.TypeUnion, vec *vector.Union) ([]vector.Any, []uint32) {
	if canonOrder(typ, vec.Values) {
		return vec.Values, vec.Tags
	}
	tagmap := make([]uint32, len(vec.Values))
	for inTag, vec := range vec.Values {
		localTag := typ.TagOf(vec.Type())
		if localTag < 0 {
			panic(sup.String(vec.Type()))
		}
		tagmap[inTag] = uint32(localTag)
	}
	tags := make([]uint32, len(vec.Tags))
	for k, intag := range vec.Tags {
		tags[k] = tagmap[intag]
	}
	vals := make([]vector.Any, len(vec.Values))
	for inTag, v := range vec.Values {
		vals[tagmap[inTag]] = v
	}
	return vals, tags
}

func canonOrder(typ *super.TypeUnion, vecs []vector.Any) bool {
	for inTag, vec := range vecs {
		if inTag != typ.TagOf(vec.Type()) {
			return false
		}
	}
	return true
}

func (u *UnionEncoder) Emit(w io.Writer) error {
	if err := u.tags.Emit(w); err != nil {
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
	u.tags.Encode(group)
	for _, value := range u.values {
		value.Encode(group)
	}
}

func (u *UnionEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, tags := u.tags.Segment(off)
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
