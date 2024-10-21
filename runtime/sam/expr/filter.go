package expr

import (
	"bytes"
	"errors"
	"net/netip"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/zcode"
)

type searchByPred struct {
	pred  Boolean
	expr  Evaluator
	types map[super.Type]bool
}

func SearchByPredicate(pred Boolean, e Evaluator) Evaluator {
	return &searchByPred{
		pred:  pred,
		expr:  e,
		types: make(map[super.Type]bool),
	}
}

func (s *searchByPred) Eval(ectx Context, val super.Value) super.Value {
	if s.expr != nil {
		val = s.expr.Eval(ectx, val)
		if val.IsError() {
			return super.False
		}
	}
	if errMatch == val.Walk(func(typ super.Type, body zcode.Bytes) error {
		if s.searchType(typ) {
			return errMatch
		}
		if s.pred(super.NewValue(typ, body)) {
			return errMatch
		}
		return nil
	}) {
		return super.True
	}
	return super.False
}

func (s *searchByPred) searchType(typ super.Type) bool {
	if match, ok := s.types[typ]; ok {
		return match
	}
	var match bool
	recType := super.TypeRecordOf(typ)
	if recType != nil {
		var nameIter FieldNameIter
		nameIter.Init(recType)
		for !nameIter.Done() {
			if s.pred(super.NewString(string(nameIter.Next()))) {
				match = true
				break
			}
		}
	}
	s.types[typ] = match
	return match
}

// stringSearch is like strings.Contains() but with case-insensitive
// comparison.
func stringSearch(a, b string) bool {
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

// NewSearch creates a filter that searches Zed records for the
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

func (s *search) Eval(ectx Context, val super.Value) super.Value {
	if s.expr != nil {
		val = s.expr.Eval(ectx, val)
		if val.IsError() {
			return super.False
		}
	}
	if errMatch == val.Walk(func(typ super.Type, body zcode.Bytes) error {
		if typ.ID() == super.IDString {
			if stringSearch(byteconv.UnsafeString(body), s.text) {
				return errMatch
			}
			return nil
		}
		if s.compare(super.NewValue(typ, body)) {
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
	bytes zcode.Bytes
}

func (s *searchCIDR) Eval(_ Context, val super.Value) super.Value {
	if errMatch == val.Walk(func(typ super.Type, body zcode.Bytes) error {
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
	term  string
	expr  Evaluator
	types map[super.Type]bool
}

// NewSearchString is like NewSeach but handles the special case of matching
// field names in addition to string values.
func NewSearchString(term string, expr Evaluator) Evaluator {
	return &searchString{
		term:  term,
		expr:  expr,
		types: make(map[super.Type]bool),
	}
}

func (s *searchString) searchType(typ super.Type) bool {
	if match, ok := s.types[typ]; ok {
		return match
	}
	var match bool
	recType := super.TypeRecordOf(typ)
	if recType != nil {
		var nameIter FieldNameIter
		nameIter.Init(recType)
		for !nameIter.Done() {
			if stringSearch(byteconv.UnsafeString(nameIter.Next()), s.term) {
				match = true
				break
			}
		}
	}
	s.types[typ] = match
	s.types[recType] = match
	return match
}

func (s *searchString) Eval(ectx Context, val super.Value) super.Value {
	if s.expr != nil {
		val = s.expr.Eval(ectx, val)
		if val.IsError() {
			return super.False
		}
	}
	// Memoize the result of a search across the names in the
	// record fields for each unique record type.
	if s.searchType(val.Type()) {
		return super.True
	}
	if errMatch == val.Walk(func(typ super.Type, body zcode.Bytes) error {
		if s.searchType(typ) {
			return errMatch
		}
		if typ.ID() == super.IDString &&
			stringSearch(byteconv.UnsafeString(body), s.term) {
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

func (f *filter) Eval(ectx Context, this super.Value) super.Value {
	val := f.expr.Eval(ectx, this)
	if val.IsError() {
		return val
	}
	if f.pred(val) {
		return super.True
	}
	return super.False
}

type filterApplier struct {
	zctx *super.Context
	expr Evaluator
}

func NewFilterApplier(zctx *super.Context, e Evaluator) Evaluator {
	return &filterApplier{zctx, e}
}

func (f *filterApplier) Eval(ectx Context, this super.Value) super.Value {
	val, ok := EvalBool(f.zctx, ectx, this, f.expr)
	if ok {
		if val.Bool() {
			return this
		}
		return f.zctx.Missing()
	}
	return val
}
