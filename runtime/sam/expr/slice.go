package expr

import (
	"errors"
	"unicode/utf8"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/scode"
)

type Slice struct {
	sctx  *super.Context
	elem  Evaluator
	from  Evaluator
	to    Evaluator
	base1 bool
}

func NewSlice(sctx *super.Context, elem, from, to Evaluator, base1 bool) *Slice {
	return &Slice{
		sctx:  sctx,
		elem:  elem,
		from:  from,
		to:    to,
		base1: base1,
	}
}

func (s *Slice) Eval(this super.Value) super.Value {
	elem := s.elem.Eval(this)
	if elem.IsNull() || elem.IsError() {
		return elem
	}
	var length int
	switch super.TypeUnder(elem.Type()).(type) {
	case *super.TypeOfBytes:
		length = len(elem.Bytes())
	case *super.TypeOfString:
		length = utf8.RuneCount(elem.Bytes())
	case *super.TypeArray, *super.TypeSet:
		n, err := elem.ContainerLength()
		if err != nil {
			panic(err)
		}
		length = n
	default:
		return s.sctx.WrapError("sliced value is not array, set, bytes, or string", elem)
	}
	from, to := 0, length
	if s.from != nil {
		val := s.sliceIndex(s.from.Eval(this), length, s.base1)
		if val.IsNull() || val.IsError() {
			return val
		}
		from = int(val.Int())
	}
	if s.to != nil {
		val := s.sliceIndex(s.to.Eval(this), length, s.base1)
		if val.IsNull() || val.IsError() {
			return val
		}
		to = int(val.Int())
	}
	from, to = FixSliceBounds(from, to, length)
	bytes := elem.Bytes()
	switch super.TypeUnder(elem.Type()).(type) {
	case *super.TypeOfBytes:
		bytes = bytes[from:to]
	case *super.TypeOfString:
		bytes = bytes[UTF8PrefixLen(bytes, from):]
		bytes = bytes[:UTF8PrefixLen(bytes, to-from)]
	case *super.TypeArray, *super.TypeSet:
		it := bytes.Iter()
		for k := 0; k < to && !it.Done(); k++ {
			if k == from {
				bytes = scode.Bytes(it)
			}
			it.Next()
		}
		bytes = bytes[:len(bytes)-len(it)]
	default:
		panic(elem.Type())
	}
	return super.NewValue(elem.Type(), bytes)
}

func (s *Slice) sliceIndex(val super.Value, length int, base1 bool) super.Value {
	if val.IsNull() || val.IsError() {
		return val
	}
	index, ok := coerce.ToInt(val, super.TypeInt64)
	if !ok {
		return s.sctx.NewError(errors.New("slice index is not a number"))
	}
	if base1 && index > 0 {
		index--
	}
	if index < 0 {
		index += int64(length)
	}
	return super.NewInt64(index)
}

func FixSliceBounds(start, end, size int) (int, int) {
	if start > end || end < 0 {
		return 0, 0
	}
	return max(start, 0), min(end, size)
}

// UTF8PrefixLen returns the length in bytes of the first runeCount runes in b.
// It returns 0 if runeCount<0 and len(b) if runeCount>utf8.RuneCount(b).
func UTF8PrefixLen(b []byte, runeCount int) int {
	var i, runeCurrent int
	for {
		if runeCurrent >= runeCount {
			return i
		}
		r, n := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && n == 0 {
			return i
		}
		i += n
		runeCurrent++
	}
}
