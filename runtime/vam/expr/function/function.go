package function

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

func New(zctx *super.Context, name string, narg int) (expr.Function, field.Path, error) {
	argmin := 1
	argmax := 1
	var path field.Path
	var f expr.Function
	switch name {
	case "abs":
		f = &Abs{zctx}
	case "base64":
		f = &Base64{zctx}
	case "bucket":
		argmin = 2
		argmax = 2
		f = &Bucket{zctx: zctx, name: name}
	case "ceil":
		f = &Ceil{zctx}
	case "coalesce":
		argmax = -1
		f = &Coalesce{}
	case "every":
		path = field.Path{"ts"}
		f = &Bucket{zctx: zctx, name: name}
	case "error":
		f = &Error{zctx}
	case "fields":
		f = NewFields(zctx)
	case "floor":
		f = &Floor{zctx}
	case "grep":
		argmax = 2
		f = &Grep{zctx: zctx}
	case "grok":
		argmin, argmax = 2, 3
		f = newGrok(zctx)
	case "hex":
		f = &Hex{zctx}
	case "join":
		argmax = 2
		f = &Join{zctx: zctx}
	case "kind":
		f = &Kind{zctx: zctx}
	case "len", "length":
		f = &Len{zctx}
	case "levenshtein":
		argmin, argmax = 2, 2
		f = &Levenshtein{zctx}
	case "log":
		f = &Log{zctx}
	case "lower":
		f = &ToLower{zctx}
	case "nest_dotted":
		path = field.Path{}
		argmin = 0
		f = &NestDotted{zctx}
	case "network_of":
		argmax = 2
		f = &NetworkOf{zctx}
	case "quiet":
		f = &Quiet{zctx}
	case "replace":
		argmin, argmax = 3, 3
		f = &Replace{zctx}
	case "round":
		f = &Round{zctx}
	case "rune_len":
		f = &RuneLen{zctx}
	case "split":
		argmin, argmax = 2, 2
		f = &Split{zctx}
	case "sqrt":
		f = &Sqrt{zctx}
	case "strftime":
		argmin, argmax = 2, 2
		f = &Strftime{zctx: zctx}
	case "trim":
		f = &Trim{zctx}
	case "typename":
		f = &TypeName{zctx: zctx}
	case "typeof":
		f = &TypeOf{zctx}
	case "upper":
		f = &ToUpper{zctx}
	case "under":
		f = &Under{zctx}
	default:
		return nil, nil, function.ErrNoSuchFunction
	}
	if err := function.CheckArgCount(narg, argmin, argmax); err != nil {
		return nil, nil, err
	}
	return f, path, nil
}

func underAll(args []vector.Any) []vector.Any {
	out := slices.Clone(args)
	for i := range args {
		out[i] = vector.Under(args[i])
	}
	return out
}
