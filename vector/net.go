package vector

import (
	"net/netip"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type Net struct {
	Values []netip.Prefix
	Nulls  bitvec.Bits
}

var _ Any = (*Net)(nil)

func NewNet(values []netip.Prefix, nulls bitvec.Bits) *Net {
	return &Net{Values: values, Nulls: nulls}
}

func (*Net) Kind() Kind {
	return KindNet
}

func (n *Net) Type() super.Type {
	return super.TypeNet
}

func (n *Net) Len() uint32 {
	return uint32(len(n.Values))
}

func (n *Net) Serialize(b *scode.Builder, slot uint32) {
	if n.Nulls.IsSet(slot) {
		b.Append(nil)
	} else {
		b.Append(super.EncodeNet(n.Values[slot]))
	}
}

func NetValue(val Any, slot uint32) (netip.Prefix, bool) {
	switch val := val.(type) {
	case *Net:
		return val.Values[slot], val.Nulls.IsSet(slot)
	case *Const:
		if val.Nulls.IsSet(slot) {
			return netip.Prefix{}, true
		}
		s, _ := val.AsBytes()
		return super.DecodeNet(s), false
	case *Dict:
		if val.Nulls.IsSet(slot) {
			return netip.Prefix{}, true
		}
		slot = uint32(val.Index[slot])
		return val.Any.(*Net).Values[slot], false
	case *View:
		slot = val.Index[slot]
		return NetValue(val.Any, slot)
	}
	panic(val)
}
