package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Const struct {
	loader NullsLoader
	val    super.Value
	length uint32
	nulls  bitvec.Bits
	loaded bool
}

var _ Any = (*Const)(nil)

func NewConst(val super.Value, length uint32, nulls bitvec.Bits) *Const {
	return &Const{val: val, length: length, nulls: nulls, loaded: true}
}

// XXX realizing we can pass in no-nulls when we know it from metadata
func NewConstWithLoader(val super.Value, loader NullsLoader, length uint32) *Const {
	return &Const{val: val, loader: loader, length: length}
}

func (c *Const) Type() super.Type {
	return c.val.Type()
}

func (c *Const) Len() uint32 {
	return c.length
}

func (*Const) Ref()   {}
func (*Const) Unref() {}

func (c *Const) Value() super.Value {
	return c.val
}

func (c *Const) Nulls() bitvec.Bits {
	if !c.loaded {
		c.nulls = c.loader.Load()
		c.loaded = true
	}
	return c.nulls
}

func (c *Const) SetNulls(nulls bitvec.Bits) {
	c.nulls = nulls
}

func (c *Const) Serialize(b *zcode.Builder, slot uint32) {
	if c.Nulls().IsSet(slot) {
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
