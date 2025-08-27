package function

import (
	"errors"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/anymath"
	"github.com/brimdata/super/runtime/sam/expr"
)

var (
	ErrBadArgument    = errors.New("bad argument")
	ErrNoSuchFunction = errors.New("no such function")
	ErrTooFewArgs     = errors.New("too few arguments")
	ErrTooManyArgs    = errors.New("too many arguments")
)

func New(sctx *super.Context, name string, narg int) (expr.Function, error) {
	argmin := 1
	argmax := 1
	var f expr.Function
	switch name {
	case "abs":
		f = &Abs{sctx: sctx}
	case "base64":
		f = &Base64{sctx: sctx}
	case "bucket":
		argmin = 2
		argmax = 2
		f = &Bucket{sctx: sctx, name: name}
	case "ceil":
		f = &Ceil{sctx: sctx}
	case "cidr_match":
		argmin = 2
		argmax = 2
		f = &CIDRMatch{sctx: sctx}
	case "coalesce":
		argmax = -1
		f = &Coalesce{}
	case "compare":
		argmin = 2
		argmax = 3
		f = NewCompare(sctx)
	case "date_part":
		argmin = 2
		argmax = 2
		f = &DatePart{sctx}
	case "error":
		f = &Error{sctx: sctx}
	case "fields":
		f = NewFields(sctx)
	case "flatten":
		f = NewFlatten(sctx)
	case "floor":
		f = &Floor{sctx: sctx}
	case "grep":
		argmin = 2
		argmax = 2
		f = &Grep{sctx: sctx}
	case "grok":
		argmin, argmax = 2, 3
		f = newGrok(sctx)
	case "has":
		argmax = -1
		f = &Has{}
	case "has_error":
		f = NewHasError()
	case "hex":
		f = &Hex{sctx: sctx}
	case "is":
		argmin = 2
		argmax = 2
		f = &Is{sctx: sctx}
	case "is_error":
		f = &IsErr{}
	case "join":
		argmax = 2
		f = &Join{sctx: sctx}
	case "kind":
		f = &Kind{sctx: sctx}
	case "ksuid":
		argmin = 0
		f = &KSUIDToString{sctx: sctx}
	case "len", "length":
		f = &LenFn{sctx: sctx}
	case "levenshtein":
		argmin = 2
		argmax = 2
		f = &Levenshtein{sctx: sctx}
	case "log":
		f = &Log{sctx: sctx}
	case "lower":
		f = &ToLower{sctx: sctx}
	case "max":
		argmax = -1
		f = &reducer{sctx: sctx, fn: anymath.Max, name: name}
	case "min":
		argmax = -1
		f = &reducer{sctx: sctx, fn: anymath.Min, name: name}
	case "missing":
		argmax = -1
		f = &Missing{}
	case "nameof":
		f = &NameOf{sctx: sctx}
	case "nest_dotted":
		f = NewNestDotted(sctx)
	case "network_of":
		argmax = 2
		f = &NetworkOf{sctx: sctx}
	case "now":
		argmax = 0
		argmin = 0
		f = &Now{}
	case "parse_sup":
		f = newParseSUP(sctx)
	case "parse_uri":
		f = NewParseURI(sctx)
	case "position":
		argmin, argmax = 2, 2
		f = &Position{sctx}
	case "pow":
		argmin = 2
		argmax = 2
		f = &Pow{sctx: sctx}
	case "quiet":
		f = &Quiet{sctx: sctx}
	case "regexp":
		argmin, argmax = 2, 2
		f = &Regexp{sctx: sctx}
	case "regexp_replace":
		argmin, argmax = 3, 3
		f = &RegexpReplace{sctx: sctx}
	case "replace":
		argmin = 3
		argmax = 3
		f = &Replace{sctx: sctx}
	case "round":
		f = &Round{sctx: sctx}
	case "rune_len":
		f = &RuneLen{sctx: sctx}
	case "split":
		argmin = 2
		argmax = 2
		f = newSplit(sctx)
	case "sqrt":
		f = &Sqrt{sctx: sctx}
	case "strftime":
		argmin, argmax = 2, 2
		f = &Strftime{sctx: sctx}
	case "trim":
		f = &Trim{sctx: sctx}
	case "typename":
		f = &typeName{sctx: sctx}
	case "typeof":
		f = &TypeOf{sctx: sctx}
	case "under":
		f = &Under{sctx: sctx}
	case "unflatten":
		f = NewUnflatten(sctx)
	case "upper":
		f = &ToUpper{sctx: sctx}
	default:
		return nil, ErrNoSuchFunction
	}
	if err := CheckArgCount(narg, argmin, argmax); err != nil {
		return nil, err
	}
	return f, nil
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
