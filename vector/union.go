package vector

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Union struct {
	*Dynamic
	Typ   *super.TypeUnion
	Nulls *Bool
}

var _ Any = (*Union)(nil)

func NewUnion(typ *super.TypeUnion, tags []uint32, vals []Any, nulls *Bool) *Union {
	d := addNullsToUnionDynamic(typ, NewDynamic(tags, vals), nulls)
	return &Union{d, typ, nulls}
}

func (u *Union) Type() super.Type {
	return u.Typ
}

func (u *Union) Serialize(b *zcode.Builder, slot uint32) {
	if vec := u.Values[u.Dynamic.Tags[slot]]; isUnionNullsVec(u.Typ, vec) {
		b.Append(nil)
		return
	}
	b.BeginContainer()
	tag := u.Typ.TagOf(u.TypeOf(slot))
	b.Append(super.EncodeInt(int64(tag)))
	u.Dynamic.Serialize(b, slot)
	b.EndContainer()
}

func Deunion(vec Any) Any {
	if union, ok := vec.(*Union); ok {
		return union.Dynamic
	}
	return vec
}

func isUnionNullsVec(typ *super.TypeUnion, vec Any) bool {
	c, ok := vec.(*Const)
	return ok && c.val.IsNull() && c.val.Type() == typ
}

func addNullsToUnionDynamic(typ *super.TypeUnion, d *Dynamic, nulls *Bool) *Dynamic {
	if nulls == nil {
		return d
	}
	nullTag := slices.IndexFunc(d.Values, func(vec Any) bool {
		return isUnionNullsVec(typ, vec)
	})
	vals := slices.Clone(d.Values)
	if nullTag == -1 {
		nullTag = len(vals)
		vals = append(vals, NewConst(super.NewValue(typ, nil), 0, nil))
	}
	var rebuild bool
	var count uint32
	delIndexes := make([][]uint32, len(vals))
	tags := slices.Clone(d.Tags)
	for i := range nulls.Len() {
		if nulls.Value(i) {
			if tags[i] != uint32(nullTag) {
				rebuild = true
				// If value was not previously null delete value from vector.
				delIndexes[tags[i]] = append(delIndexes[tags[i]], d.TagMap.Forward[i])
			}
			tags[i] = uint32(nullTag)
			count++
		}
	}
	vals[nullTag].(*Const).len = count
	if rebuild {
		for i, delIndex := range delIndexes {
			if len(delIndex) > 0 {
				vals[i] = NewInverseView(vals[i], delIndex)
			}
		}
		return NewDynamic(tags, vals)
	}
	return d
}
