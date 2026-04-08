package vector

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Union struct {
	*Dynamic
	Typ *super.TypeUnion
}

var _ Any = (*Union)(nil)

func NewUnion(typ *super.TypeUnion, tags []uint32, vals []Any) *Union {
	return &Union{NewDynamic(tags, vals), typ}
}

func NewUnionFromDynamic(sctx *super.Context, d *Dynamic) *Union {
	types := make([]super.Type, 0, len(d.Values))
	for _, vec := range d.Values {
		types = append(types, vec.Type())
	}
	unionType, ok := sctx.LookupTypeUnion(types)
	if !ok {
		panic(types)
	}
	return &Union{d, unionType}
}

func (*Union) Kind() Kind {
	return KindUnion
}

func (u *Union) Type() super.Type {
	return u.Typ
}

func (u *Union) Serialize(b *scode.Builder, slot uint32) {
	tag := u.Typ.TagOf(u.TypeOf(slot))
	super.BeginUnion(b, tag)
	u.Dynamic.Serialize(b, slot)
	b.EndContainer()
}

func Deunion(vec Any) Any {
	if u, ok := vec.(*Union); ok {
		return u.Dynamic
	}
	return vec
}

// FlattenUnions takes a Dynamic and recursively replaces any Union values
// with their inner values, rewriting tags so that each slot points directly
// to the leaf value vector.
func FlattenUnions(d *Dynamic) *Dynamic {
	hasUnion := slices.ContainsFunc(d.Values, func(vec Any) bool {
		_, ok := vec.(*Union)
		return ok
	})
	if !hasUnion {
		return d
	}
	bases := make([]uint32, len(d.Values))
	unions := make([]*Dynamic, len(d.Values))
	var newValues []Any
	for i, val := range d.Values {
		bases[i] = uint32(len(newValues))
		if u, ok := val.(*Union); ok {
			flat := FlattenUnions(u.Dynamic)
			unions[i] = flat
			newValues = append(newValues, flat.Values...)
		} else {
			newValues = append(newValues, val)
		}
	}
	forward := d.ForwardTagMap()
	newTags := make([]uint32, len(d.Tags))
	for slot, oldTag := range d.Tags {
		base := bases[oldTag]
		if d := unions[oldTag]; d != nil {
			innerSlot := forward[slot]
			newTags[slot] = base + d.Tags[innerSlot]
		} else {
			newTags[slot] = base
		}
	}
	return NewDynamic(newTags, newValues)
}
