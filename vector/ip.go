package vector

import (
	"net/netip"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type IP struct {
	Values []netip.Addr
	Nulls  bitvec.Bits
}

var _ Any = (*IP)(nil)

func NewIP(values []netip.Addr, nulls bitvec.Bits) *IP {
	return &IP{Values: values, Nulls: nulls}
}

func (*IP) Kind() Kind {
	return KindIP
}

func (i *IP) Type() super.Type {
	return super.TypeIP
}

func (i *IP) Len() uint32 {
	return uint32(len(i.Values))
}

func (i *IP) Serialize(b *scode.Builder, slot uint32) {
	if i.Nulls.IsSet(slot) {
		b.Append(nil)
	} else {
		b.Append(super.EncodeIP(i.Values[slot]))
	}
}

func IPValue(val Any, slot uint32) (netip.Addr, bool) {
	switch val := val.(type) {
	case *IP:
		return val.Values[slot], val.Nulls.IsSet(slot)
	case *Const:
		if val.Nulls.IsSet(slot) {
			return netip.Addr{}, true
		}
		b, _ := val.AsBytes()
		return super.DecodeIP(b), false
	case *Dict:
		if val.Nulls.IsSet(slot) {
			return netip.Addr{}, true
		}
		slot = uint32(val.Index[slot])
		return val.Any.(*IP).Values[slot], false
	case *View:
		slot = val.Index[slot]
		return IPValue(val.Any, slot)
	}
	panic(val)
}
