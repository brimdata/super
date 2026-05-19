package vbuild

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
)

type unionBuilder struct {
	typ       *super.TypeUnion
	values    []Builder
	rleOrTags []uint32
}

func newUnionBuilder(typ *super.TypeUnion) Builder {
	values := make([]Builder, len(typ.Types))
	for i, typ := range typ.Types {
		values[i] = New(typ)
	}
	return &unionBuilder{typ: typ, values: values}
}

func (u *unionBuilder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	union := vec.(*vector.Union)
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
		vecs, rle = reorderRLE(union, rle)
		u.rleOrTags = concatRLEs(u.rleOrTags, rle)
	} else {
		var tags []uint32
		vecs, tags = reorder(union)
		u.rleOrTags = append(u.rleOrTags, tags...)
	}
	for k, vec := range vecs {
		if vec != nil && vec.Len() != 0 {
			u.values[k].Write(vec)
		}
	}
}

func concatRLEs(a, b []uint32) []uint32 {
	if a == nil {
		return b
	}
	runsA := len(a)
	firstA := 1
	if a[0] == 0 {
		firstA = 0
		runsA--
	}
	lastA := firstA
	if runsA%2 == 0 {
		lastA = 1 - firstA
	}
	firstB := 1
	if b[0] == 0 {
		firstB = 0
		b = b[1:] // Trim opening zero from b.
	}
	out := slices.Clone(a)
	if lastA == firstB {
		// Merge last run in a with first run in B.
		out[len(out)-1] += b[0]
		return append(out, b[1:]...)
	}
	return append(out, b...)
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

func (u *unionBuilder) Build(sctx *super.Context) vector.Any {
	vals := make([]vector.Any, len(u.typ.Types))
	for i, b := range u.values {
		vals[i] = b.Build(sctx)
	}
	if len(u.typ.Types) == 2 {
		return vector.NewUnionFromRLE(u.typ, u.rleOrTags, vals)
	}
	return vector.NewUnion(u.typ, u.rleOrTags, vals)
}
