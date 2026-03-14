package vector

import (
	"fmt"
	"net/netip"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Const struct {
	Any
	len uint32
}

func NewConst(vec Any, length uint32) *Const {
	return &Const{vec, length}
}

func NewConstUint(typ super.Type, v uint64, length uint32) *Const {
	vec := NewUint(typ, []uint64{v})
	return &Const{vec, length}
}
func NewConstInt(typ super.Type, v int64, length uint32) *Const {
	vec := NewInt(typ, []int64{v})
	return &Const{vec, length}
}
func NewConstFloat(typ super.Type, v float64, length uint32) *Const {
	vec := NewFloat(typ, []float64{v})
	return &Const{vec, length}
}

var (
	falseVec = NewFalse(1)
	trueVec  = NewTrue(1)
)

func NewConstBool(v bool, length uint32) *Const {
	if v {
		return &Const{trueVec, length}
	}
	return &Const{falseVec, length}
}

func NewConstBytes(v []byte, length uint32) *Const {
	vec := NewBytes(newBytesTableWithValue(v))
	return &Const{vec, length}
}

func newBytesTableWithValue(b []byte) BytesTable {
	offsets := []uint32{0, uint32(len(b))}
	return NewBytesTable(offsets, b)
}

func NewConstString(v string, length uint32) *Const {
	vec := NewString(newBytesTableWithValue([]byte(v)))
	return &Const{vec, length}
}

func NewConstIP(v netip.Addr, length uint32) *Const {
	vec := NewIP([]netip.Addr{v})
	return &Const{vec, length}
}

func NewConstNet(v netip.Prefix, length uint32) *Const {
	vec := NewNet([]netip.Prefix{v})
	return &Const{vec, length}
}
func NewConstType(v []byte, length uint32) *Const {
	vec := NewTypeValue(newBytesTableWithValue(v))
	return &Const{vec, length}
}

func NewConstFromValue(val super.Value, length uint32) *Const {
	switch id := val.Type().ID(); {
	case super.IsUnsigned(id):
		return NewConstUint(val.Type(), val.Uint(), length)
	case super.IsSigned(id):
		return NewConstInt(val.Type(), val.Int(), length)
	case super.IsFloat(id):
		return NewConstFloat(val.Type(), val.Float(), length)
	case id == super.IDBool:
		return NewConstBool(val.Bool(), length)
	case id == super.IDBytes:
		return NewConstBytes(val.Bytes(), length)
	case id == super.IDString:
		return NewConstString(string(val.Bytes()), length)
	case id == super.IDIP:
		return NewConstIP(super.DecodeIP(val.Bytes()), length)
	case id == super.IDNet:
		return NewConstNet(super.DecodeNet(val.Bytes()), length)
	case id == super.IDType:
		return NewConstType(val.Bytes(), length)
	}
	panic(fmt.Sprintf("%#v\n", super.TypeUnder(val.Type())))
}

func (c *Const) Len() uint32 {
	return c.len
}

func (c *Const) Serialize(b *scode.Builder, slot uint32) {
	if slot >= c.len {
		panic([]uint32{slot, c.len})
	}
	c.Any.Serialize(b, 0)
}
