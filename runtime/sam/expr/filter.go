package expr

import (
	"bytes"
	"errors"
	"net/netip"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/scode"
)

type searchByPred struct {
	pred Boolean
	expr Evaluator
	fnm  *FieldNameMatcher
}

func SearchByPredicate(pred Boolean, e Evaluator) Evaluator {
	return &searchByPred{
		pred: pred,
		expr: e,
		fnm: NewFieldNameMatcher(func(b []byte) bool {
			val := super.NewValue(super.TypeString, b)
			return pred(val).Ptr().AsBool()
		}),
	}
}

func (s *searchByPred) Eval(val super.Value) super.Value {
	if s.expr != nil {
		val = s.expr.Eval(val)
		if val.IsError() {
			return super.False
		}
	}
	if s.fnm.Match(val.Type()) {
		return super.True
	}
	if errMatch == val.Walk(func(typ super.Type, body scode.Bytes) error {
		if s.pred(super.NewValue(typ, body)).Ptr().AsBool() {
			return errMatch
		}
		return nil
	}) {
		return super.True
	}
	return super.False
}

// StringContainsFold is like strings.Contains but with case-insensitive
// comparison.
func StringContainsFold(a, b string) bool {
	alen := len(a)
	blen := len(b)

	if blen > alen {
		return false
	}

	end := alen - blen + 1
	i := 0
	for i < end {
		if strings.EqualFold(a[i:i+blen], b) {
			return true
		}
		i++
	}
	return false
}

var errMatch = errors.New("match")

type search struct {
	text    string
	compare Boolean
	expr    Evaluator
}

// NewSearch creates a filter that searches records for the
// given value, which must be of a type other than string.  The filter
// matches a record that contains this value either as the value of any
// field or inside any set or array.  It also matches a record if the string
// representaton of the search value appears inside inside any string-valued
// field (or inside any element of a set or array of strings).
func NewSearch(searchtext string, searchval super.Value, expr Evaluator) (Evaluator, error) {
	if super.TypeUnder(searchval.Type()) == super.TypeNet {
		return &searchCIDR{
			net:   super.DecodeNet(searchval.Bytes()),
			bytes: searchval.Bytes(),
		}, nil
	}
	typedCompare, err := Comparison("==", searchval)
	if err != nil {
		return nil, err
	}
	return &search{searchtext, typedCompare, expr}, nil
}

func (s *search) Eval(val super.Value) super.Value {
	if s.expr != nil {
		val = s.expr.Eval(val)
		if val.IsError() {
			return super.False
		}
	}
	if errMatch == val.Walk(func(typ super.Type, body scode.Bytes) error {
		if typ.ID() == super.IDString {
			if StringContainsFold(byteconv.UnsafeString(body), s.text) {
				return errMatch
			}
			return nil
		}
		if s.compare(super.NewValue(typ, body)).Ptr().AsBool() {
			return errMatch
		}
		return nil
	}) {
		return super.True
	}
	return super.False
}

type searchCIDR struct {
	net   netip.Prefix
	bytes scode.Bytes
}

func (s *searchCIDR) Eval(val super.Value) super.Value {
	if errMatch == val.Walk(func(typ super.Type, body scode.Bytes) error {
		switch typ.ID() {
		case super.IDNet:
			if bytes.Equal(body, s.bytes) {
				return errMatch
			}
		case super.IDIP:
			if s.net.Contains(super.DecodeIP(body)) {
				return errMatch
			}
		}
		return nil
	}) {
		return super.True
	}
	return super.False
}

type searchString struct {
	term string
	expr Evaluator
	fnm  *FieldNameMatcher
}

// NewSearchString is like NewSeach but handles the special case of matching
// field names in addition to string values.
func NewSearchString(term string, expr Evaluator) Evaluator {
	return &searchString{
		term: term,
		expr: expr,
		fnm: NewFieldNameMatcher(func(b []byte) bool {
			return StringContainsFold(byteconv.UnsafeString(b), term)
		}),
	}
}

func (s *searchString) Eval(val super.Value) super.Value {
	if s.expr != nil {
		val = s.expr.Eval(val)
		if val.IsError() {
			return super.False
		}
	}
	if s.fnm.Match(val.Type()) {
		return super.True
	}
	if errMatch == val.Walk(func(typ super.Type, body scode.Bytes) error {
		if typ.ID() == super.IDString &&
			StringContainsFold(byteconv.UnsafeString(body), s.term) {
			return errMatch
		}
		return nil
	}) {
		return super.True
	}
	return super.False
}

type filter struct {
	expr Evaluator
	pred Boolean
}

func NewFilter(expr Evaluator, pred Boolean) Evaluator {
	return &filter{expr, pred}
}

func (f *filter) Eval(this super.Value) super.Value {
	val := f.expr.Eval(this)
	if val.IsError() {
		return val
	}
	return f.pred(val)
}

type filterApplier struct {
	sctx *super.Context
	expr Evaluator
}

func NewFilterApplier(sctx *super.Context, e Evaluator) Evaluator {
	return &filterApplier{sctx, e}
}

func (f *filterApplier) Eval(this super.Value) super.Value {
	val := EvalBool(f.sctx, this, f.expr)
	if val.Type().ID() == super.IDBool {
		if val.Bool() {
			return this
		}
		return f.sctx.Missing()
	}
	return val
}
