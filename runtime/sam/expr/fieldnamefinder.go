package expr

import (
	"encoding/binary"
	"math/big"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/stringsearch"
	"github.com/brimdata/super/zcode"
)

type FieldNameFinder struct {
	checkedIDs big.Int
	fnm        *FieldNameMatcher
}

func NewFieldNameFinder(pattern string) *FieldNameFinder {
	caseFinder := stringsearch.NewCaseFinder(pattern)
	pred := func(b []byte) bool {
		return caseFinder.Next(byteconv.UnsafeString(b)) != -1
	}
	return &FieldNameFinder{fnm: NewFieldNameMatcher(pred)}
}

// Find returns true if buf, which holds a sequence of ZNG value messages, might
// contain a record with a field whose fully-qualified name (e.g., a.b.c)
// matches the pattern.  Find also returns true if it encounters an error.
func (f *FieldNameFinder) Find(types super.TypeFetcher, buf []byte) bool {
	f.checkedIDs.SetInt64(0)
	clear(f.fnm.checkedIDs)
	for len(buf) > 0 {
		id, idLen := binary.Uvarint(buf)
		if idLen <= 0 {
			return true
		}
		valLen := zcode.DecodeTagLength(buf[idLen:])
		buf = buf[idLen+valLen:]
		if f.checkedIDs.Bit(int(id)) == 1 {
			continue
		}
		f.checkedIDs.SetBit(&f.checkedIDs, int(id), 1)
		t, err := types.LookupType(int(id))
		if err != nil {
			return true
		}
		if f.fnm.Match(t) {
			return true
		}
	}
	return false
}

type FieldNameMatcher struct {
	pred       func([]byte) bool
	checkedIDs map[int]bool
	fni        FieldNameIter
}

func NewFieldNameMatcher(pred func([]byte) bool) *FieldNameMatcher {
	return &FieldNameMatcher{pred: pred, checkedIDs: map[int]bool{}}
}

func (f *FieldNameMatcher) Match(typ super.Type) bool {
	id := typ.ID()
	match, ok := f.checkedIDs[id]
	if ok {
		return match
	}
	switch typ := super.TypeUnder(typ).(type) {
	case *super.TypeRecord:
		for f.fni.Init(typ); !f.fni.Done() && !match; {
			match = f.pred(f.fni.Next())
		}
		if match {
			break
		}
		for _, field := range typ.Fields {
			match = f.Match(field.Type)
			if match {
				break
			}
		}
	case *super.TypeArray:
		match = f.Match(typ.Type)
	case *super.TypeSet:
		match = f.Match(typ.Type)
	case *super.TypeMap:
		match = f.Match(typ.KeyType)
		if !match {
			match = f.Match(typ.ValType)
		}
	case *super.TypeUnion:
		for _, t := range typ.Types {
			match = f.Match(t)
			if match {
				break
			}
		}
	case *super.TypeError:
		match = f.Match(typ.Type)
	}
	f.checkedIDs[id] = match
	return match
}

type FieldNameIter struct {
	buf   []byte
	stack []fieldNameIterInfo
}

type fieldNameIterInfo struct {
	fields []super.Field
	offset int
}

func (f *FieldNameIter) Init(t *super.TypeRecord) {
	f.buf = f.buf[:0]
	f.stack = f.stack[:0]
	if len(t.Fields) > 0 {
		f.stack = append(f.stack, fieldNameIterInfo{t.Fields, 0})
	}
}

func (f *FieldNameIter) Done() bool {
	return len(f.stack) == 0
}

func (f *FieldNameIter) Next() []byte {
	// Step into non-empty records.
	for {
		info := &f.stack[len(f.stack)-1]
		field := info.fields[info.offset]
		f.buf = append(f.buf, "."+field.Name...)
		t, ok := super.TypeUnder(field.Type).(*super.TypeRecord)
		if !ok || len(t.Fields) == 0 {
			break
		}
		f.stack = append(f.stack, fieldNameIterInfo{t.Fields, 0})
	}
	// Skip leading dot.
	name := f.buf[1:]
	// Advance our position and step out of records.
	for len(f.stack) > 0 {
		info := &f.stack[len(f.stack)-1]
		field := info.fields[info.offset]
		f.buf = f.buf[:len(f.buf)-len(field.Name)-1]
		info.offset++
		if info.offset < len(info.fields) {
			break
		}
		f.stack = f.stack[:len(f.stack)-1]
	}
	return name
}
