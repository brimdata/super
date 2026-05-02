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
	count  uint32
	tags   []uint32
	tagEnc *Uint32Encoder
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
	var vecs []vector.Any
	if len(union.Typ.Types) == 2 {
		// Code tags as run lengths.
		rle := union.TagsRLE()
		if rle == nil {
			// Encoder returns nil for all tag 0
			rle = []uint32{0, vec.Len()}
		}
		// RLEs have the nice property that you can just concatenate them
		// to append two vectors.
		vecs, rle = reorderRLE(union, rle)
		u.tags = append(u.tags, rle...)
	} else {
		var tags []uint32
		vecs, tags = reorder(union)
		u.tags = append(u.tags, tags...)
	}
	for k, vec := range vecs {
		if vec != nil && vec.Len() != 0 {
			u.values[k].Write(vec)
		}
	}
}

func reorderRLE(union *vector.Union, rle []uint32) ([]vector.Any, []uint32) {
	vecs := union.Values()
	if canonOrder(union.Typ, vecs) {
		return vecs, rle
	}
	if rle[0] == 0 {
		rle = rle[1:]
	} else {
		rle = append([]uint32{0}, rle...)
	}
	return []vector.Any{vecs[1], vecs[0]}, rle
}

func reorder(union *vector.Union) ([]vector.Any, []uint32) {
	vecs := union.Values()
	if canonOrder(union.Typ, vecs) {
		return vecs, union.Tags()
	}
	tagmap := make([]uint32, len(vecs))
	for inTag, vec := range vecs {
		localTag := union.Typ.TagOf(vec.Type())
		if localTag < 0 {
			panic(sup.String(vec.Type()))
		}
		tagmap[inTag] = uint32(localTag)
	}
	tags := make([]uint32, len(union.Tags()))
	for k, intag := range union.Tags() {
		tags[k] = tagmap[intag]
	}
	vals := make([]vector.Any, len(union.Typ.Types))
	for inTag, v := range union.Values() {
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
	if err := u.tagEnc.Emit(w); err != nil {
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
	u.tagEnc = &Uint32Encoder{vals: u.tags}
	u.tagEnc.Encode(group)
	for _, value := range u.values {
		value.Encode(group)
	}
}

func (u *UnionEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, tags := u.tagEnc.Segment(off)
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
