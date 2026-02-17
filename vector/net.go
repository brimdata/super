package vector

import (
	"net/netip"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Net struct {
	Values []netip.Prefix
}

var _ Any = (*Net)(nil)

func NewNet(values []netip.Prefix) *Net {
	return &Net{values}
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
	b.Append(super.EncodeNet(n.Values[slot]))
}

func NetValue(val Any, slot uint32) netip.Prefix {
	switch val := val.(type) {
	case *Net:
		return val.Values[slot]
	case *Const:
		s, _ := val.AsBytes()
		return super.DecodeNet(s)
	case *Dict:
		slot = uint32(val.Index[slot])
		return val.Any.(*Net).Values[slot]
	case *View:
		slot = val.Index[slot]
		return NetValue(val.Any, slot)
	}
	panic(val)
}
