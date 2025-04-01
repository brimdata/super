package vector

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Union struct {
	*Dynamic
	Typ *super.TypeUnion
}

var _ Any = (*Union)(nil)

func NewUnion(typ *super.TypeUnion, tags []uint32, vals []Any) *Union {
	return &Union{NewDynamic(tags, vals), typ}
}

func (u *Union) Type() super.Type {
	return u.Typ
}

func (u *Union) Serialize(b *zcode.Builder, slot uint32) {
	var builder zcode.Builder
	u.Dynamic.Serialize(&builder, slot)
	bytes := builder.Bytes().Body()
	typ := u.TypeOf(slot)
	if bytes == nil && typ == u.Typ {
		b.Append(nil)
		return
	}
	b.BeginContainer()
	tag := u.Typ.TagOf(typ)
	b.Append(super.EncodeInt(int64(tag)))
	b.Append(bytes)
	b.EndContainer()
}

func (u *Union) Nulls() *Bool {
	nullsTag := u.getNullsIndex()
	if nullsTag == -1 {
		return nil
	}
	b := NewBoolEmpty(u.Len(), nil)
	for i, tag := range u.Tags {
		if tag == uint32(nullsTag) {
			b.Set(uint32(i))
		}
	}
	return b
}

func (u *Union) getNullsIndex() int {
	return slices.IndexFunc(u.Values, func(vec Any) bool {
		var ok bool
		c, ok := vec.(*Const)
		return ok && c.val.IsNull() && c.val.Type() == u.Typ
	})
}

func Deunion(vec Any) Any {
	if union, ok := vec.(*Union); ok {
		return union.Dynamic
	}
	return vec
}

func addNullsToUnion(u *Union, nulls *Bool) *Union {
	if nulls == nil {
		return u
	}
	vals := slices.Clone(u.Values)
	nullTag := u.getNullsIndex()
	if nullTag == -1 {
		nullTag = len(u.Values)
		vals = append(vals, NewConst(super.NewValue(u.Typ, nil), 0, nil))
	}
	var count uint32
	tags := slices.Clone(u.Tags)
	for i, tag := range u.Tags {
		if tag == uint32(nullTag) || nulls.Value(uint32(i)) {
			tags[i] = uint32(nullTag)
			count++
		}
	}
	vals[nullTag].(*Const).len = count
	return NewUnion(u.Typ, tags, vals)
}
