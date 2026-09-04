package expr

import (
	"regexp"
	"regexp/syntax"

	"github.com/brimdata/super"
)

// Boolean is a function that takes a Value and returns a boolean result
// based on the typed value.
type Boolean func(super.Value) super.Value

func CompileRegexp(pattern string) (*regexp.Regexp, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		if syntaxErr, ok := err.(*syntax.Error); ok {
			syntaxErr.Expr = pattern
		}
		return nil, err
	}
	return re, err
}
