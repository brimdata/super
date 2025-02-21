package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/zcode"
)

type Const struct {
	val   super.Value
	len   uint32
	Nulls *Bool
}

var _ Any = (*Const)(nil)

func NewConst(val super.Value, len uint32, nulls *Bool) *Const {
	return &Const{val: val, len: len, Nulls: nulls}
}

func (c *Const) Type() super.Type {
	return c.val.Type()
}

func (c *Const) Len() uint32 {
	return c.len
}

func (*Const) Ref()   {}
func (*Const) Unref() {}

func (c *Const) Length() int {
	return int(c.len)
}

func (c *Const) Value() super.Value {
	return c.val
}

func (c *Const) Serialize(b *zcode.Builder, slot uint32) {
	if c.Nulls.Value(slot) {
		b.Append(nil)
	} else {
		b.Append(c.val.Bytes())
	}
}

func (c *Const) AsBytes() ([]byte, bool) {
	return c.val.Bytes(), c.val.Type().ID() == super.IDBytes
}

func (c *Const) AsFloat() (float64, bool) {
	return coerce.ToFloat(c.val, super.TypeFloat64)
}

func (c *Const) AsInt() (int64, bool) {
	return coerce.ToInt(c.val, super.TypeInt64)
}

func (c *Const) AsUint() (uint64, bool) {
	return coerce.ToUint(c.val, super.TypeUint64)
}

func (c *Const) AsString() (string, bool) {
	return c.val.AsString(), c.val.IsString()
}
