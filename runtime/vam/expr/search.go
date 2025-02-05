package expr

import (
	"net/netip"
	"regexp"
	"slices"
	"unsafe"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/vector"
)

type search struct {
	e          Evaluator
	vectorPred func(vector.Any) vector.Any
	stringPred func([]byte) bool
	fnm        *expr.FieldNameMatcher
}

func NewSearch(s string, val super.Value, e Evaluator) Evaluator {
	stringPred := func(b []byte) bool {
		return expr.StringContainsFold(string(b), s)
	}
	var net netip.Prefix
	if val.Type().ID() == super.IDNet {
		net = super.DecodeNet(val.Bytes())
	}
	eq := NewCompare(super.NewContext() /* XXX */, nil, nil, "==")
	vectorPred := func(vec vector.Any) vector.Any {
		if net.IsValid() && vector.KindOf(vec) == vector.KindIP {
			out := vector.NewBoolEmpty(vec.Len(), nil)
			for i := range vec.Len() {
				if ip, null := vector.IPValue(vec, i); !null && net.Contains(ip) {
					out.Set(i)
				}
			}
			return out
		}
		return eq.eval(vec, vector.NewConst(val, vec.Len(), nil))
	}
	return &search{e, vectorPred, stringPred, nil}
}

func NewSearchRegexp(re *regexp.Regexp, e Evaluator) Evaluator {
	return &search{e, nil, re.Match, expr.NewFieldNameMatcher(re.Match)}
}

func NewSearchString(s string, e Evaluator) Evaluator {
	pred := func(b []byte) bool {
		return expr.StringContainsFold(string(b), s)
	}
	return &search{e, nil, pred, expr.NewFieldNameMatcher(pred)}
}

func (s *search) Eval(this vector.Any) vector.Any {
	return vector.Apply(true, s.eval, s.e.Eval(this))
}

func (s *search) eval(vecs ...vector.Any) vector.Any {
	vec := vector.Under(vecs[0])
	typ := vec.Type()
	if s.fnm != nil && s.fnm.Match(typ) {
		return vector.NewConst(super.True, vec.Len(), nil)
	}
	if typ.Kind() == super.PrimitiveKind {
		return s.match(vec)
	}
	n := vec.Len()
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		vec = view.Any
		index = view.Index
	}
	switch vec := vec.(type) {
	case *vector.Record:
		out := vector.NewBoolEmpty(n, nil)
		for _, f := range vec.Fields {
			if index != nil {
				f = vector.NewView(f, index)
			}
			out = vector.Or(out, toBool(s.eval(f)))
		}
		return out
	case *vector.Array:
		return s.evalForList(vec.Values, vec.Offsets, index, n)
	case *vector.Set:
		return s.evalForList(vec.Values, vec.Offsets, index, n)
	case *vector.Map:
		return vector.Or(s.evalForList(vec.Keys, vec.Offsets, index, n),
			s.evalForList(vec.Values, vec.Offsets, index, n))
	case *vector.Union:
		return vector.Apply(true, s.eval, vec)
	case *vector.Error:
		return s.eval(vec.Vals)
	}
	panic(vec)
}

func (s *search) evalForList(vec vector.Any, offsets, index []uint32, length uint32) *vector.Bool {
	out := vector.NewBoolEmpty(length, nil)
	var index2 []uint32
	for j := range length {
		if index != nil {
			j = index[j]
		}
		start, end := offsets[j], offsets[j+1]
		if start == end {
			continue
		}
		n := end - start
		index2 = slices.Grow(index2[:0], int(n))[:n]
		for k := range n {
			index2[k] = k + start
		}
		view := vector.NewView(vec, index2)
		if toBool(s.eval(view)).TrueCount() > 0 {
			out.Set(j)
		}
	}
	return out
}

func (s *search) match(vec vector.Any) vector.Any {
	if vec.Type().ID() == super.IDString {
		out := vector.NewBoolEmpty(vec.Len(), nil)
		for i := range vec.Len() {
			str, null := vector.StringValue(vec, i)
			// Prevent compiler from copying str, which it thinks
			// escapes to the heap because stringPred is a pointer.
			bytes := unsafe.Slice(unsafe.StringData(str), len(str))
			if !null && s.stringPred(bytes) {
				out.Set(i)
			}
		}
		return out
	}
	if s.vectorPred != nil {
		return s.vectorPred(vec)
	}
	return vector.NewConst(super.False, vec.Len(), nil)
}

type regexpMatch struct {
	re *regexp.Regexp
	e  Evaluator
}

func NewRegexpMatch(re *regexp.Regexp, e Evaluator) Evaluator {
	return &regexpMatch{re, e}
}

func (r *regexpMatch) Eval(this vector.Any) vector.Any {
	return vector.Apply(true, r.eval, r.e.Eval(this))
}

func (r *regexpMatch) eval(vecs ...vector.Any) vector.Any {
	vec := vector.Under(vecs[0])
	if c, ok := vec.(*vector.Const); ok && c.Value().Type().ID() == super.IDNull {
		return vector.NewConst(super.NullBool, vec.Len(), nil)
	}
	if vec.Type().ID() != super.IDString {
		return vector.NewConst(super.False, vec.Len(), nil)
	}
	out := vector.NewBoolEmpty(vec.Len(), nil)
	for i := range vec.Len() {
		s, isnull := vector.StringValue(vec, i)
		if isnull {
			if out.Nulls == nil {
				out.Nulls = vector.NewBoolEmpty(vec.Len(), nil)
			}
			out.Nulls.Set(i)
			continue
		}
		if r.re.MatchString(s) {
			out.Set(i)
		}
	}
	return out
}
