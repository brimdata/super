package function

import (
	"errors"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/anymath"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zson"
)

var (
	ErrBadArgument    = errors.New("bad argument")
	ErrNoSuchFunction = errors.New("no such function")
	ErrTooFewArgs     = errors.New("too few arguments")
	ErrTooManyArgs    = errors.New("too many arguments")
)

func New(zctx *super.Context, name string, narg int) (expr.Function, field.Path, error) {
	argmin := 1
	argmax := 1
	var path field.Path
	var f expr.Function
	switch name {
	default:
		return nil, nil, ErrNoSuchFunction
	case "coalesce":
		argmax = -1
		f = &Coalesce{}
	case "grep":
		argmax = 2
		f = &Grep{zctx: zctx}
	case "grok":
		argmin, argmax = 2, 3
		f = newGrok(zctx)
	case "len", "length":
		f = &LenFn{zctx: zctx}
	case "abs":
		f = &Abs{zctx: zctx}
	case "every":
		path = field.Path{"ts"}
		f = &Bucket{
			zctx: zctx,
			name: "every",
		}
	case "ceil":
		f = &Ceil{zctx: zctx}
	case "flatten":
		f = NewFlatten(zctx)
	case "floor":
		f = &Floor{zctx: zctx}
	case "join":
		argmax = 2
		f = &Join{zctx: zctx}
	case "ksuid":
		argmin = 0
		f = &KSUIDToString{zctx: zctx}
	case "levenshtein":
		argmin = 2
		argmax = 2
		f = &Levenshtein{zctx: zctx}
	case "log":
		f = &Log{zctx: zctx}
	case "max":
		argmax = -1
		f = &reducer{zctx: zctx, fn: anymath.Max, name: name}
	case "min":
		argmax = -1
		f = &reducer{zctx: zctx, fn: anymath.Min, name: name}
	case "now":
		argmax = 0
		argmin = 0
		f = &Now{}
	case "round":
		f = &Round{zctx: zctx}
	case "pow":
		argmin = 2
		argmax = 2
		f = &Pow{zctx: zctx}
	case "sqrt":
		f = &Sqrt{zctx: zctx}
	case "replace":
		argmin = 3
		argmax = 3
		f = &Replace{zctx: zctx}
	case "rune_len":
		f = &RuneLen{zctx: zctx}
	case "lower":
		f = &ToLower{zctx: zctx}
	case "upper":
		f = &ToUpper{zctx: zctx}
	case "trim":
		f = &Trim{zctx: zctx}
	case "split":
		argmin = 2
		argmax = 2
		f = newSplit(zctx)
	case "bucket":
		argmin = 2
		argmax = 2
		f = &Bucket{zctx: zctx, name: name}
	case "typename":
		f = &typeName{zctx: zctx}
	case "typeof":
		f = &TypeOf{zctx: zctx}
	case "nameof":
		f = &NameOf{zctx: zctx}
	case "fields":
		f = NewFields(zctx)
	case "has":
		argmax = -1
		f = &Has{}
	case "has_error":
		f = NewHasError()
	case "is":
		argmin = 1
		argmax = 2
		path = field.Path{}
		f = &Is{zctx: zctx}
	case "is_error":
		f = &IsErr{}
	case "error":
		f = &Error{zctx: zctx}
	case "kind":
		f = &Kind{zctx: zctx}
	case "base64":
		f = &Base64{zctx: zctx}
	case "hex":
		f = &Hex{zctx: zctx}
	case "compare":
		argmin = 2
		argmax = 3
		f = NewCompare(zctx)
	case "cidr_match":
		argmin = 2
		argmax = 2
		f = &CIDRMatch{zctx: zctx}
	case "missing":
		argmax = -1
		f = &Missing{}
	case "network_of":
		argmax = 2
		f = &NetworkOf{zctx: zctx}
	case "nest_dotted":
		path = field.Path{}
		argmin = 0
		f = NewNestDotted(zctx)
	case "parse_uri":
		f = &ParseURI{zctx: zctx, marshaler: zson.NewZNGMarshalerWithContext(zctx)}
	case "parse_zson":
		f = newParseZSON(zctx)
	case "quiet":
		f = &Quiet{zctx: zctx}
	case "regexp":
		argmin, argmax = 2, 2
		f = &Regexp{zctx: zctx}
	case "regexp_replace":
		argmin, argmax = 3, 3
		f = &RegexpReplace{zctx: zctx}
	case "strftime":
		argmin, argmax = 2, 2
		f = &Strftime{zctx: zctx}
	case "under":
		f = &Under{zctx: zctx}
	case "unflatten":
		f = NewUnflatten(zctx)
	}
	if err := CheckArgCount(narg, argmin, argmax); err != nil {
		return nil, nil, err
	}
	return f, path, nil
}

func CheckArgCount(narg int, argmin int, argmax int) error {
	if argmin != -1 && narg < argmin {
		return ErrTooFewArgs
	}
	if argmax != -1 && narg > argmax {
		return ErrTooManyArgs
	}
	return nil
}

// HasBoolResult returns true if the function name returns a Boolean value.
// XXX This is a hack so the semantic compiler can determine if a single call
// expr is a Filter or Put proc. At some point function declarations should have
// signatures so the return type can be introspected.
func HasBoolResult(name string) bool {
	switch name {
	case "grep", "has", "has_error", "is_error", "is", "missing", "cidr_match":
		return true
	}
	return false
}

func underAll(args []super.Value) []super.Value {
	for i := range args {
		args[i] = args[i].Under()
	}
	return args
}
