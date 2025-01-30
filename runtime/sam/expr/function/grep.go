package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"golang.org/x/text/unicode/norm"
)

type Grep struct {
	grep    expr.Evaluator
	pattern string
	zctx    *super.Context
}

func (g *Grep) Call(_ super.Allocator, vals []super.Value) super.Value {
	patternVal, inputVal := vals[0], vals[1]
	if super.TypeUnder(patternVal.Type()) != super.TypeString {
		return g.zctx.WrapError("grep(): pattern argument must be a string", patternVal)
	}
	if patternVal.IsNull() {
		return super.NullBool
	}
	if p := patternVal.AsString(); g.grep == nil || g.pattern != p {
		g.pattern = p
		term := norm.NFC.Bytes(patternVal.Bytes())
		g.grep = expr.NewSearchString(string(term), nil)
	}
	return g.grep.Eval(expr.NewContext(), inputVal)
}
