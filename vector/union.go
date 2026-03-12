package vector

import (
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
	return &Union{d, sctx.LookupTypeUnion(types)}
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
