package expr

import (
	"errors"
	"unicode/utf8"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/scode"
)

type Slice struct {
	sctx *super.Context
	elem Evaluator
	from Evaluator
	to   Evaluator
	sql  bool
}

func NewSlice(sctx *super.Context, elem, from, to Evaluator, sql bool) *Slice {
	return &Slice{
		sctx: sctx,
		elem: elem,
		from: from,
		to:   to,
		sql:  sql,
	}
}

var ErrSliceIndex = errors.New("slice index is not a number")
var ErrSliceIndexEmpty = errors.New("slice index is empty")

func (s *Slice) Eval(this super.Value) super.Value {
	elem := s.elem.Eval(this)
	if elem.IsError() {
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
	if elem.IsNull() {
		return elem
	}
	from, err := sliceIndex(this, s.from, length, s.sql)
	if err != nil && err != ErrSliceIndexEmpty {
		return s.sctx.NewError(err)
	}
	to, err := sliceIndex(this, s.to, length, s.sql)
	if err != nil {
		if err != ErrSliceIndexEmpty {
			return s.sctx.NewError(err)
		}
		to = length
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

func sliceIndex(this super.Value, slot Evaluator, length int, sql bool) (int, error) {
	if slot == nil {
		//XXX
		return 0, ErrSliceIndexEmpty
	}
	val := slot.Eval(this)
	v, ok := coerce.ToInt(val, super.TypeInt64)
	if !ok {
		return 0, ErrSliceIndex
	}
	index := int(v)
	if sql && index > 0 {
		index--
	}
	if index < 0 {
		index += length
	}
	return index, nil
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
