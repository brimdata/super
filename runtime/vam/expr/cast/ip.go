package cast

import (
	"net/netip"

	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/vector"
)

func castToIP(vec vector.Any, index []uint32) (vector.Any, []uint32, string, bool) {
	switch vec := vec.(type) {
	case *vector.IP:
		return vec, nil, "", true
	case *vector.String:
		n := lengthOf(vec, index)
		var ips []netip.Addr
		var errs []uint32
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			ip, err := byteconv.ParseIP(vec.Table().Bytes(idx))
			if err != nil {
				errs = append(errs, i)
				continue
			}
			ips = append(ips, ip)
		}
		return vector.NewIP(ips), errs, "", true
	default:
		return nil, nil, "", false
	}
}

func castToNet(vec vector.Any, index []uint32) (vector.Any, []uint32, string, bool) {
	switch vec := vec.(type) {
	case *vector.Net:
		return vec, nil, "", true
	case *vector.String:
		n := lengthOf(vec, index)
		var nets []netip.Prefix
		var errs []uint32
		for i := range n {
			idx := i
			if index != nil {
				idx = index[i]
			}
			net, err := netip.ParsePrefix(vec.Value(idx))
			if err != nil {
				errs = append(errs, i)
				continue
			}
			nets = append(nets, net)
		}
		return vector.NewNet(nets), errs, "", true
	default:
		return nil, nil, "", false
	}
}
