package expr

import (
	"fmt"
	"unicode/utf8"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/stringsearch"
)

const (
	opAnd = iota
	opOr
	opFieldNameFinder
	opStringCaseFinder
	opStringFinder
)

// BufferFilter is a filter for byte slices containing BSUP-serialized values.
type BufferFilter struct {
	op    int
	left  *BufferFilter
	right *BufferFilter
	fnf   *FieldNameFinder
	cf    *stringsearch.CaseFinder
	f     *stringsearch.Finder
}

func NewAndBufferFilter(left, right *BufferFilter) *BufferFilter {
	return &BufferFilter{op: opAnd, left: left, right: right}
}

func NewOrBufferFilter(left, right *BufferFilter) *BufferFilter {
	return &BufferFilter{op: opOr, left: left, right: right}
}

func NewBufferFilterForFieldName(pattern string) *BufferFilter {
	return &BufferFilter{
		op:  opFieldNameFinder,
		fnf: NewFieldNameFinder(pattern),
	}
}

func NewBufferFilterForString(pattern string) *BufferFilter {
	if len(pattern) < 2 {
		// Very short patterns are unprofitable.
		return nil
	}
	return &BufferFilter{op: opStringFinder, f: stringsearch.NewFinder(pattern)}
}

func NewBufferFilterForStringCase(pattern string) *BufferFilter {
	if len(pattern) < 2 {
		// Very short patterns are unprofitable.
		return nil
	}
	for _, r := range pattern {
		if r >= utf8.RuneSelf {
			// stringCaseFinder is sensitive to case for letters
			// with multibyte UTF-8 encodings.
			return nil
		}
	}
	return &BufferFilter{op: opStringCaseFinder, cf: stringsearch.NewCaseFinder(pattern)}
}

// Eval returns true if buf matches the receiver and false otherwise.
func (b *BufferFilter) Eval(types super.TypeFetcher, buf []byte) bool {
	switch b.op {
	case opAnd:
		return b.left.Eval(types, buf) && b.right.Eval(types, buf)
	case opOr:
		return b.left.Eval(types, buf) || b.right.Eval(types, buf)
	case opFieldNameFinder:
		return b.fnf.Find(types, buf)
	case opStringCaseFinder:
		return b.cf.Next(byteconv.UnsafeString(buf)) > -1
	case opStringFinder:
		return b.f.Next(byteconv.UnsafeString(buf)) > -1
	default:
		panic(fmt.Sprintf("BufferFilter: unknown op %d", b.op))
	}
}
