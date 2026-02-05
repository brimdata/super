package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"golang.org/x/text/unicode/norm"
)

type Grep struct {
	sctx    *super.Context
	grep    expr.Evaluator
	pattern string
}

func (g *Grep) Call(args ...vector.Any) vector.Any {
	patternVec, inputVec := args[0], args[1]
	if patternVec.Type().ID() != super.IDString {
		return vector.NewWrappedError(g.sctx, "grep: pattern argument must be a string", patternVec)
	}
	if inputVec.Len() == 0 {
		return vector.NewFalse(0)
	}
	if c, ok := vector.Under(patternVec).(*vector.Const); ok {
		pattern, _ := c.AsString()
		if g.grep == nil || g.pattern != pattern {
			pattern = norm.NFC.String(pattern)
			g.grep = expr.NewSearchString(g.sctx, pattern, &expr.This{})
			g.pattern = pattern
		}
		return g.grep.Eval(inputVec)
	}
	var index [1]uint32
	out := vector.NewFalse(patternVec.Len())
	for i := range patternVec.Len() {
		pattern := vector.StringValue(patternVec, i)
		pattern = norm.NFC.String(pattern)
		search := expr.NewSearchString(g.sctx, pattern, &expr.This{})
		index[0] = i
		if vector.BoolValue(search.Eval(vector.Pick(inputVec, index[:])), 0) {
			out.Set(i)
		}
	}
	return out
}
