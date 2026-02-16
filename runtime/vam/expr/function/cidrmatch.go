package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type CIDRMatch struct {
	sctx *super.Context
	pw   *expr.PredicateWalk
}

func NewCIDRMatch(sctx *super.Context) *CIDRMatch {
	return &CIDRMatch{sctx, expr.NewPredicateWalk(sctx, cidrMatch)}
}

func (c *CIDRMatch) Call(args ...vector.Any) vector.Any {
	if id := args[0].Type().ID(); id != super.IDNet && id != super.IDNull {
		return vector.NewWrappedError(c.sctx, "cidr_match: not a net", args[0])
	}
	return c.pw.Eval(args...)
}

func cidrMatch(vec ...vector.Any) vector.Any {
	if vec, ok := expr.CheckNulls(vec); ok {
		return vec
	}
	netVec, valVec := vec[0], vec[1]
	if id := valVec.Type().ID(); id != super.IDIP {
		return vector.NewConst(super.False, valVec.Len())
	}
	out := vector.NewFalse(valVec.Len())
	for i := range netVec.Len() {
		net := vector.NetValue(netVec, i)
		ip := vector.IPValue(valVec, i)
		if net.Contains(ip) {
			out.Set(i)
		}
	}
	return out
}
