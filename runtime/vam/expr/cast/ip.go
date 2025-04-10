package cast

import (
	"net/netip"

	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

func castToIP(vec vector.Any, index []uint32) (vector.Any, []uint32, bool) {
	switch vec := vec.(type) {
	case *vector.IP:
		return vec, nil, true
	case *vector.String:
		n := lengthOf(vec, index)
		var nulls bitvec.Bits
		var ips []netip.Addr
		var errs []uint32
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			if vec.Nulls.IsSet(idx) {
				if nulls.IsZero() {
					nulls = bitvec.NewFalse(n)
				}
				nulls.Set(i)
				ips = append(ips, netip.Addr{})
				continue
			}
			ip, err := byteconv.ParseIP(vec.Bytes[vec.Offsets[idx]:vec.Offsets[idx+1]])
			if err != nil {
				errs = append(errs, i)
				continue
			}
			ips = append(ips, ip)
		}
		return vector.NewIP(ips, nulls), errs, true
	default:
		return nil, nil, false
	}
}

func castToNet(vec vector.Any, index []uint32) (vector.Any, []uint32, bool) {
	switch vec := vec.(type) {
	case *vector.Net:
		return vec, nil, true
	case *vector.String:
		n := lengthOf(vec, index)
		var nulls bitvec.Bits
		var nets []netip.Prefix
		var errs []uint32
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			if vec.Nulls.IsSet(idx) {
				if nulls.IsZero() {
					nulls = bitvec.NewFalse(n)
				}
				nulls.Set(i)
				nets = append(nets, netip.Prefix{})
				continue
			}
			net, err := netip.ParsePrefix(vec.Value(idx))
			if err != nil {
				errs = append(errs, i)
				continue
			}
			nets = append(nets, net)
		}
		return vector.NewNet(nets, nulls), errs, true
	default:
		return nil, nil, false
	}
}
