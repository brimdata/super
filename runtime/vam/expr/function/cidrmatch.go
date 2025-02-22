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
	if id := args[0].Type().ID(); id != super.IDNet && id != super.IDNull {
		out := vector.NewWrappedError(c.zctx, "cidr_match: not a net", args[0])
		out.Nulls = vector.Or(vector.NullsOf(args[0]), vector.NullsOf(args[1]))
		return out
	}
	return c.pw.Eval(args...)
}

func cidrMatch(vec ...vector.Any) vector.Any {
	netVec, valVec := vec[0], vec[1]
	nulls := vector.Or(vector.NullsOf(netVec), vector.NullsOf(valVec))
	if id := valVec.Type().ID(); id != super.IDIP {
		return vector.NewConst(super.False, valVec.Len(), nulls)
	}
	out := vector.NewBoolEmpty(valVec.Len(), nulls)
	for i := range netVec.Len() {
		net, null := vector.NetValue(netVec, i)
		if null {
			continue
		}
		ip, null := vector.IPValue(valVec, i)
		if null {
			continue
		}
		if net.Contains(ip) {
			out.Set(i)
		}
	}
	return out
}
