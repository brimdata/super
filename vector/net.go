package vector

import (
	"net/netip"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Net struct {
	Values []netip.Prefix
	Nulls  *Bool
}

var _ Any = (*Net)(nil)

func NewNet(values []netip.Prefix, nulls *Bool) *Net {
	return &Net{Values: values, Nulls: nulls}
}

func (n *Net) Type() zed.Type {
	return zed.TypeNet
}

func (n *Net) Len() uint32 {
	return uint32(len(n.Values))
}

func (n *Net) Serialize(b *zcode.Builder, slot uint32) {
	if n.Nulls.Value(slot) {
		b.Append(nil)
	} else {
		b.Append(zed.EncodeNet(n.Values[slot]))
	}
}

func (n *Net) AppendKey(b []byte, slot uint32) []byte {
	if n.Nulls.Value(slot) {
		return append(b, 0)
	}
	return zed.AppendNet(b, n.Values[slot])
}

func NetValue(val Any, slot uint32) (netip.Prefix, bool) {
	switch val := val.(type) {
	case *Net:
		return val.Values[slot], val.Nulls.Value(slot)
	case *Const:
		if val.Nulls.Value(slot) {
			return netip.Prefix{}, true
		}
		s, _ := val.AsBytes()
		return zed.DecodeNet(s), false
	case *Dict:
		if val.Nulls.Value(slot) {
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
