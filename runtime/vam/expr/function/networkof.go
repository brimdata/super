package function

import (
	"net/netip"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#network_of
type NetworkOf struct {
	sctx *super.Context
}

func (n *NetworkOf) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	ipvec := args[0]
	if ipvec.Type().ID() != super.IDIP {
		return vector.NewWrappedError(n.sctx, "network_of: not an IP", ipvec)
	}
	if len(args) == 1 {
		return n.singleIP(ipvec)
	}
	maskvec := args[1]
	switch id := maskvec.Type().ID(); {
	case id == super.IDIP:
		return n.ipMask(ipvec, maskvec)
	case super.IsInteger(id):
		return n.intMask(ipvec, maskvec)
	default:
		return vector.NewWrappedError(n.sctx, "network_of: bad arg for CIDR mask", maskvec)
	}
}

func (n *NetworkOf) singleIP(vec vector.Any) vector.Any {
	if c, ok := vec.(*vector.Const); ok {
		ip, _ := vector.IPValue(vec, 0)
		if !ip.Is4() {
			return errNotIP4(n.sctx, vec)
		}
		net := netip.PrefixFrom(ip, bitsFromIP(ip.As4())).Masked()
		return vector.NewConst(super.NewNet(net), c.Len(), c.Nulls)
	}
	var errs []uint32
	var nets vector.Any
	switch vec := vec.(type) {
	case *vector.IP:
		nets, errs = n.singleIPLoop(vec, nil)
	case *vector.View:
		nets, errs = n.singleIPLoop(vec.Any.(*vector.IP), vec.Index)
	case *vector.Dict:
		netVals, derrs := n.singleIPLoop(vec.Any.(*vector.IP), nil)
		index, counts, nulls := vec.Index, vec.Counts, vec.Nulls
		if len(derrs) > 0 {
			index, counts, nulls, errs = vec.RebuildDropTags(derrs...)
		}
		nets = vector.NewDict(netVals, index, counts, nulls)
	}
	if len(errs) > 0 {
		return vector.Combine(nets, errs, errNotIP4(n.sctx, vector.Pick(vec, errs)))
	}
	return nets
}

func (n *NetworkOf) singleIPLoop(vec *vector.IP, index []uint32) (*vector.Net, []uint32) {
	var nets []netip.Prefix
	var errs []uint32
	for i := range vec.Len() {
		idx := i
		if index != nil {
			idx = index[i]
		}
		ip := vec.Values[idx]
		if !ip.Is4() {
			errs = append(errs, i)
			continue
		}
		nets = append(nets, netip.PrefixFrom(ip, bitsFromIP(ip.As4())).Masked())
	}
	return vector.NewNet(nets, nil), errs
}

// inlined
func bitsFromIP(b [4]byte) int {
	switch {
	case b[0] < 0x80:
		return 8
	case b[0] < 0xc0:
		return 16
	default:
		return 24
	}
}

func (n *NetworkOf) ipMask(ipvec, maskvec vector.Any) vector.Any {
	var nets []netip.Prefix
	var errsLen, errsCont []uint32
	for i := range ipvec.Len() {
		ip, _ := vector.IPValue(ipvec, i)
		mask, _ := vector.IPValue(maskvec, i)
		if mask.BitLen() != ip.BitLen() {
			errsLen = append(errsLen, i)
			continue
		}
		bits := super.LeadingOnes(mask.AsSlice())
		if netip.PrefixFrom(mask, bits).Masked().Addr() != mask {
			errsCont = append(errsCont, i)
			continue
		}
		nets = append(nets, netip.PrefixFrom(ip, bits).Masked())
	}
	b := vector.NewCombiner(vector.NewNet(nets, nil))
	m := addressAndMask(n.sctx, ipvec, maskvec)
	b.WrappedError(n.sctx, errsLen, "network_of: address and mask have different lengths", m)
	b.WrappedError(n.sctx, errsCont, "network_of: mask is non-contiguous", maskvec)
	return b.Result()
}

func (n *NetworkOf) intMask(ipvec, maskvec vector.Any) vector.Any {
	var errs []uint32
	var out vector.Any
	if c, ok := maskvec.(*vector.Const); ok {
		bits, _ := c.AsInt()
		if _, ok := ipvec.(*vector.Const); ok {
			ip, _ := vector.IPValue(ipvec, 0)
			net := netip.PrefixFrom(ip, int(bits))
			if net.Bits() < 0 {
				return errCIDRRange(n.sctx, ipvec, maskvec)
			}
			return vector.NewConst(super.NewNet(net.Masked()), ipvec.Len(), nil)
		}
		out, errs = n.intMaskFast(ipvec, int(bits))
	} else {
		id := maskvec.Type().ID()
		var nets []netip.Prefix
		for i := range ipvec.Len() {
			var bits int
			if super.IsSigned(id) {
				b, _ := vector.IntValue(maskvec, i)
				bits = int(b)
			} else {
				b, _ := vector.UintValue(maskvec, i)
				bits = int(b)
			}
			ip, _ := vector.IPValue(ipvec, i)
			net := netip.PrefixFrom(ip, bits)
			if net.Bits() < 0 {
				errs = append(errs, i)
				continue
			}
			nets = append(nets, netip.PrefixFrom(ip, bits).Masked())
		}
		out = vector.NewNet(nets, nil)
	}
	if len(errs) > 0 {
		m := vector.Pick(addressAndMask(n.sctx, ipvec, maskvec), errs)
		err := vector.NewWrappedError(n.sctx, "network_of: CIDR bit count out of range", m)
		return vector.Combine(out, errs, err)
	}
	return out
}

func (n *NetworkOf) intMaskFast(vec vector.Any, bits int) (vector.Any, []uint32) {
	switch vec := vec.(type) {
	case *vector.IP:
		return n.intMaskFastLoop(vec, nil, bits)
	case *vector.View:
		return n.intMaskFastLoop(vec.Any.(*vector.IP), vec.Index, bits)
	case *vector.Dict:
		nets, derrs := n.intMaskFastLoop(vec.Any.(*vector.IP), nil, bits)
		index, counts, nulls := vec.Index, vec.Counts, vec.Nulls
		var errs []uint32
		if len(derrs) > 0 {
			index, counts, nulls, errs = vec.RebuildDropTags(derrs...)
		}
		return vector.NewDict(nets, index, counts, nulls), errs
	default:
		panic(vec)
	}
}

func (n *NetworkOf) intMaskFastLoop(vec *vector.IP, index []uint32, bits int) (vector.Any, []uint32) {
	var errs []uint32
	var nets []netip.Prefix
	for i := range vec.Len() {
		idx := i
		if index != nil {
			idx = index[i]
		}
		ip := vec.Values[idx]
		net := netip.PrefixFrom(ip, bits)
		if net.Bits() < 0 {
			errs = append(errs, i)
			continue
		}
		nets = append(nets, net.Masked())
	}
	return vector.NewNet(nets, nil), errs
}

func errCIDRRange(sctx *super.Context, ipvec, maskvec vector.Any) vector.Any {
	vec := addressAndMask(sctx, ipvec, maskvec)
	return vector.NewWrappedError(sctx, "network_of: CIDR bit count out of range", vec)
}

func errNotIP4(sctx *super.Context, vec vector.Any) vector.Any {
	return vector.NewWrappedError(sctx, "network_of: not an IPv4 address", vec)
}

func addressAndMask(sctx *super.Context, address, mask vector.Any) vector.Any {
	typ := sctx.MustLookupTypeRecord([]super.Field{
		{Name: "address", Type: address.Type()},
		{Name: "mask", Type: mask.Type()},
	})
	return vector.NewRecord(typ, []vector.Any{address, mask}, address.Len(), nil)
}
