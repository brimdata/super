package vector

import (
	"encoding/binary"

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
	return &Union{NewDynamic(tags, vals), typ, nulls}
}

func (u *Union) Type() super.Type {
	return u.Typ
}

func (u *Union) Serialize(b *zcode.Builder, slot uint32) {
	if u.Nulls.Value(slot) {
		b.Append(nil)
		return
	}
	b.BeginContainer()
	tag := u.Typ.TagOf(u.TypeOf(slot))
	b.Append(super.EncodeInt(int64(tag)))
	u.Dynamic.Serialize(b, slot)
	b.EndContainer()
}

func (u *Union) AppendKey(b []byte, slot uint32) []byte {
	b = binary.NativeEndian.AppendUint64(b, uint64(u.Typ.ID()))
	if u.Nulls.Value(slot) {
		return append(b, 0)
	}
	return u.Dynamic.AppendKey(b, slot)
}

func Deunion(vec Any) Any {
	if union, ok := vec.(*Union); ok {
		// XXX if the Union has Nulls this will be broken.
		return union.Dynamic
	}
	return vec
}
