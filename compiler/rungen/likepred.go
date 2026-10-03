package rungen

import (
	"bytes"
	"regexp/syntax"
)

// PROTOTYPE: LIKE is translated to a regexp. For the common pattern shapes
// 'abc%', '%abc', and '%abc%', the generated regexp is ^abc.*?$, ^.*?abc$,
// or ^.*?abc.*?$ (with the (?s) flag).  Recognize those and use a plain
// byte comparison instead of running the regexp engine on every value.
// Anything else (e.g., '_' wildcards or multiple '%' between literals)
// returns nil so the caller falls back to the regexp.
func likePredicate(pattern string) func([]byte) bool {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil || re.Op != syntax.OpConcat {
		return nil
	}
	subs := re.Sub
	if len(subs) < 3 || subs[0].Op != syntax.OpBeginText || subs[len(subs)-1].Op != syntax.OpEndText {
		return nil
	}
	mid := subs[1 : len(subs)-1]
	isAny := func(r *syntax.Regexp) bool {
		return r.Op == syntax.OpStar && r.Sub[0].Op == syntax.OpAnyChar
	}
	lit := func(r *syntax.Regexp) ([]byte, bool) {
		if r.Op != syntax.OpLiteral || r.Flags&syntax.FoldCase != 0 {
			return nil, false
		}
		return []byte(string(r.Rune)), true
	}
	switch len(mid) {
	case 2:
		if s, ok := lit(mid[0]); ok && isAny(mid[1]) {
			return func(b []byte) bool { return bytes.HasPrefix(b, s) }
		}
		if s, ok := lit(mid[1]); ok && isAny(mid[0]) {
			return func(b []byte) bool { return bytes.HasSuffix(b, s) }
		}
	case 3:
		if s, ok := lit(mid[1]); ok && isAny(mid[0]) && isAny(mid[2]) {
			return func(b []byte) bool { return bytes.Contains(b, s) }
		}
	}
	return nil
}
