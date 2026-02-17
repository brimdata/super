package vector

import (
	"net/netip"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type IP struct {
	Values []netip.Addr
}

func NewIP(values []netip.Addr) *IP {
	return &IP{values}
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
	b.Append(super.EncodeIP(i.Values[slot]))
}

func IPValue(val Any, slot uint32) netip.Addr {
	switch val := val.(type) {
	case *IP:
		return val.Values[slot]
	case *Const:
		b, _ := val.AsBytes()
		return super.DecodeIP(b)
	case *Dict:
		slot = uint32(val.Index[slot])
		return val.Any.(*IP).Values[slot]
	case *View:
		slot = val.Index[slot]
		return IPValue(val.Any, slot)
	}
	panic(val)
}
