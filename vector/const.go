package vector

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type Const struct {
	val   super.Value
	len   uint32
	Nulls bitvec.Bits
}

var _ Any = (*Const)(nil)

func NewConst(val super.Value, len uint32, nulls bitvec.Bits) *Const {
	if val.IsNull() {
		nulls = bitvec.NewTrue(len)
	}
	return &Const{val: val, len: len, Nulls: nulls}
}

func (c *Const) Kind() Kind {
	// c.val must be a primitive.
	switch id := c.val.Type().ID(); {
	case super.IsUnsigned(id):
		return KindUint
	case super.IsSigned(id):
		return KindInt
	case super.IsFloat(id):
		return KindFloat
	case id == super.IDBool:
		return KindBool
	case id == super.IDBytes:
		return KindBytes
	case id == super.IDString:
		return KindString
	case id == super.IDIP:
		return KindIP
	case id == super.IDNet:
		return KindNet
	case id == super.IDType:
		return KindType
	case id == super.IDNull:
		return KindNull
	}
	panic(fmt.Sprintf("%#v\n", super.TypeUnder(c.val.Type())))
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

func (c *Const) Serialize(b *scode.Builder, slot uint32) {
	if c.Nulls.IsSet(slot) {
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
