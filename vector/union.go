package vector

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type Union struct {
	*Dynamic
	Typ   *super.TypeUnion
	Nulls bitvec.Bits
}

var _ Any = (*Union)(nil)

func NewUnion(typ *super.TypeUnion, tags []uint32, vals []Any, nulls bitvec.Bits) *Union {
	return &Union{NewDynamic(tags, vals), typ, nulls}
}

func (u *Union) Type() super.Type {
	return u.Typ
}

func (u *Union) Serialize(b *scode.Builder, slot uint32) {
	if u.Nulls.IsSet(slot) {
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
	if u, ok := vec.(*Union); ok {
		return addUnionNullsToDynamic(u.Typ, NewDynamic(u.Tags, u.Values), u.Nulls)
	}
	return vec
}

func isUnionNullsVec(typ *super.TypeUnion, vec Any) bool {
	c, ok := vec.(*Const)
	return ok && c.val.IsNull() && c.val.Type() == typ
}

func addUnionNullsToDynamic(typ *super.TypeUnion, d *Dynamic, nulls bitvec.Bits) *Dynamic {
	if nulls.IsZero() {
		return d
	}
	nullTag := slices.IndexFunc(d.Values, func(vec Any) bool {
		return isUnionNullsVec(typ, vec)
	})
	vals := slices.Clone(d.Values)
	if nullTag == -1 {
		nullTag = len(vals)
		vals = append(vals, NewConst(super.NewValue(typ, nil), 0, bitvec.Zero))
	}
	var rebuild bool
	var count uint32
	delIndexes := make([][]uint32, len(vals))
	tags := slices.Clone(d.Tags)
	forward := d.ForwardTagMap()
	for i := range nulls.Len() {
		if nulls.IsSetDirect(i) {
			if tags[i] != uint32(nullTag) {
				rebuild = true
				// If value was not previously null delete value from vector.
				delIndexes[tags[i]] = append(delIndexes[tags[i]], forward[i])
			}
			tags[i] = uint32(nullTag)
			count++
		}
	}
	vals[nullTag].(*Const).len = count
	if rebuild {
		for i, delIndex := range delIndexes {
			if len(delIndex) > 0 {
				vals[i] = ReversePick(vals[i], delIndex)
			}
		}
		return NewDynamic(tags, vals)
	}
	return d
}
