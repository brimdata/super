package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
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
			g.grep = expr.NewSearchString(pattern, &expr.This{})
			g.pattern = pattern
		}
		return g.grep.Eval(inputVec)
	}
	var index [1]uint32
	nulls := bitvec.Or(vector.NullsOf(patternVec), vector.NullsOf(inputVec))
	out := vector.NewBoolEmpty(patternVec.Len(), nulls)
	for i := range patternVec.Len() {
		if nulls.IsSet(i) {
			continue
		}
		pattern, _ := vector.StringValue(patternVec, i)
		pattern = norm.NFC.String(pattern)
		search := expr.NewSearchString(pattern, &expr.This{})
		index[0] = i
		if match, _ := vector.BoolValue(search.Eval(vector.Pick(inputVec, index[:])), 0); match {
			out.Set(i)
		}
	}
	return out
}
