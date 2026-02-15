package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Union struct {
	*Dynamic
	Typ *super.TypeUnion
}

func NewUnion(typ *super.TypeUnion, tags []uint32, vals []Any) *Union {
	return &Union{NewDynamic(tags, vals), typ}
}

func (*Union) Kind() Kind {
	return KindUnion
}

func (u *Union) Type() super.Type {
	return u.Typ
}

func (u *Union) Serialize(b *scode.Builder, slot uint32) {
	b.BeginContainer()
	tag := u.Typ.TagOf(u.TypeOf(slot))
	b.Append(super.EncodeInt(int64(tag)))
	u.Dynamic.Serialize(b, slot)
	b.EndContainer()
}

func Deunion(vec Any) Any {
	if u, ok := vec.(*Union); ok {
		return u.Dynamic
	}
	return vec
}
