package function

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

func New(sctx *super.Context, name string, narg int) (expr.Function, field.Path, error) {
	argmin := 1
	argmax := 1
	var path field.Path
	var f expr.Function
	switch name {
	case "abs":
		f = &Abs{sctx}
	case "base64":
		f = &Base64{sctx}
	case "bucket":
		argmin = 2
		argmax = 2
		f = &Bucket{sctx: sctx, name: name}
	case "ceil":
		f = &Ceil{sctx}
	case "cidr_match":
		argmin = 2
		argmax = 2
		f = NewCIDRMatch(sctx)
	case "coalesce":
		argmax = -1
		f = &Coalesce{}
	case "date_part":
		argmin = 2
		argmax = 2
		f = &DatePart{sctx}
	case "every":
		path = field.Path{"ts"}
		f = &Bucket{sctx: sctx, name: name}
	case "error":
		f = &Error{sctx}
	case "fields":
		f = NewFields(sctx)
	case "flatten":
		f = newFlatten(sctx)
	case "floor":
		f = &Floor{sctx}
	case "grep":
		argmin = 2
		argmax = 2
		f = &Grep{sctx: sctx}
	case "grok":
		argmin, argmax = 2, 3
		f = newGrok(sctx)
	case "has":
		argmax = -1
		f = newHas(sctx)
	case "hex":
		f = &Hex{sctx}
	case "is":
		argmin = 1
		argmax = 2
		path = field.Path{}
		f = &Is{sctx: sctx}
	case "join":
		argmax = 2
		f = &Join{sctx: sctx}
	case "kind":
		f = &Kind{sctx: sctx}
	case "len", "length":
		f = &Len{sctx}
	case "levenshtein":
		argmin, argmax = 2, 2
		f = &Levenshtein{sctx}
	case "log":
		f = &Log{sctx}
	case "lower":
		f = &ToLower{sctx}
	case "missing":
		argmax = -1
		f = &Missing{}
	case "nameof":
		f = &NameOf{sctx: sctx}
	case "nest_dotted":
		path = field.Path{}
		argmin = 0
		f = &NestDotted{sctx}
	case "now":
		path = field.Path{}
		argmax = 0
		argmin = 0
		f = &Now{}
	case "network_of":
		argmax = 2
		f = &NetworkOf{sctx}
	case "parse_sup":
		f = newParseSUP(sctx)
	case "parse_uri":
		f = newParseURI(sctx)
	case "position":
		argmin, argmax = 2, 2
		f = &Position{sctx}
	case "pow":
		argmin = 2
		argmax = 2
		f = &Pow{sctx}
	case "quiet":
		f = &Quiet{sctx}
	case "regexp":
		argmin, argmax = 2, 2
		f = &Regexp{sctx: sctx}
	case "regexp_replace":
		argmin, argmax = 3, 3
		f = &RegexpReplace{sctx: sctx}
	case "replace":
		argmin, argmax = 3, 3
		f = &Replace{sctx}
	case "round":
		f = &Round{sctx}
	case "rune_len":
		f = &RuneLen{sctx}
	case "split":
		argmin, argmax = 2, 2
		f = &Split{sctx}
	case "sqrt":
		f = &Sqrt{sctx}
	case "strftime":
		argmin, argmax = 2, 2
		f = &Strftime{sctx: sctx}
	case "trim":
		f = &Trim{sctx}
	case "typename":
		f = &TypeName{sctx: sctx}
	case "typeof":
		f = &TypeOf{sctx}
	case "under":
		f = &Under{sctx}
	case "unflatten":
		f = newUnflatten(sctx)
	case "upper":
		f = &ToUpper{sctx}
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
