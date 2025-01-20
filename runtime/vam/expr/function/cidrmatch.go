package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type CIDRMatch struct {
	zctx *super.Context
	pw   *expr.PredicateWalk
}

func NewCIDRMatch(zctx *super.Context) *CIDRMatch {
	return &CIDRMatch{zctx, expr.NewPredicateWalk(cidrMatch)}
}

func (c *CIDRMatch) Call(args ...vector.Any) vector.Any {
	if args[0].Type().ID() != super.IDNet {
		return vector.NewWrappedError(c.zctx, "cidr_match: not a net", args[0])
	}
	return c.pw.Eval(args...)
}

func cidrMatch(vec ...vector.Any) vector.Any {
	netVec := vec[0]
	ipVec := vec[1]
	if ipVec.Type().ID() != super.IDIP {
		return vector.NewConst(super.False, ipVec.Len(), nil)
	}
	out := vector.NewBoolEmpty(netVec.Len(), nil)
	for i := range netVec.Len() {
		net, null := vector.NetValue(netVec, i)
		if null {
			continue
		}
		ip, null := vector.IPValue(ipVec, i)
		if null {
			continue
		}
		if net.Contains(ip) {
			out.Set(i)
		}
	}
	return out
}
