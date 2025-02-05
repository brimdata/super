package function

import (
	"errors"
	"net/netip"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zson"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#network_of
type NetworkOf struct {
	zctx *super.Context
}

func (n *NetworkOf) Call(_ super.Allocator, args []super.Value) super.Value {
	id := args[0].Type().ID()
	if id != super.IDIP {
		return n.zctx.WrapError("network_of: not an IP", args[0])
	}
	ip := super.DecodeIP(args[0].Bytes())
	var bits int
	if len(args) == 1 {
		switch {
		case !ip.Is4():
			return n.zctx.WrapError("network_of: not an IPv4 address", args[0])
		case ip.As4()[0] < 0x80:
			bits = 8
		case ip.As4()[0] < 0xc0:
			bits = 16
		default:
			bits = 24
		}
	} else {
		// two args
		body := args[1].Bytes()
		switch id := args[1].Type().ID(); {
		case id == super.IDIP:
			mask := super.DecodeIP(body)
			if mask.BitLen() != ip.BitLen() {
				return n.zctx.WrapError("network_of: address and mask have different lengths", addressAndMask(args[0], args[1]))
			}
			bits = super.LeadingOnes(mask.AsSlice())
			if netip.PrefixFrom(mask, bits).Masked().Addr() != mask {
				return n.zctx.WrapError("network_of: mask is non-contiguous", args[1])
			}
		case super.IsInteger(id):
			if super.IsSigned(id) {
				bits = int(args[1].Int())
			} else {
				bits = int(args[1].Uint())
			}
			if bits > 128 || bits > 32 && ip.Is4() {
				return n.zctx.WrapError("network_of: CIDR bit count out of range", addressAndMask(args[0], args[1]))
			}
		default:
			return n.zctx.WrapError("network_of: bad arg for CIDR mask", args[1])
		}
	}
	// Mask for canonical form.
	prefix := netip.PrefixFrom(ip, bits).Masked()
	return super.NewNet(prefix)
}

func addressAndMask(address, mask super.Value) super.Value {
	val, err := zson.MarshalZNG(struct {
		Address super.Value `zed:"address"`
		Mask    super.Value `zed:"mask"`
	}{address, mask})
	if err != nil {
		panic(err)
	}
	return val
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#cidr_match
type CIDRMatch struct {
	zctx *super.Context
}

var errMatch = errors.New("match")

func (c *CIDRMatch) Call(_ super.Allocator, args []super.Value) super.Value {
	maskVal := args[0]
	if id := maskVal.Type().ID(); id != super.IDNet && id != super.IDNull {
		val := c.zctx.WrapError("cidr_match: not a net", maskVal)
		if maskVal.IsNull() {
			val = super.NewValue(val.Type(), nil)
		}
		return val

	}
	if maskVal.IsNull() || args[1].IsNull() {
		return super.NewValue(super.TypeBool, nil)
	}
	prefix := super.DecodeNet(maskVal.Bytes())
	err := args[1].Walk(func(typ super.Type, body zcode.Bytes) error {
		if typ.ID() == super.IDIP {
			if prefix.Contains(super.DecodeIP(body)) {
				return errMatch
			}
		}
		return nil
	})
	return super.NewBool(err == errMatch)
}
