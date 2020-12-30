package filter

import (
	"encoding/binary"
	"math/big"

	"github.com/brimsec/zq/pkg/byteconv"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
)

type fieldNameFinder struct {
	checkedIDs       big.Int
	fieldNameIter    fieldNameIter
	stringCaseFinder *stringCaseFinder
}

func newFieldNameFinder(pattern string) *fieldNameFinder {
	return &fieldNameFinder{stringCaseFinder: makeStringCaseFinder(pattern)}
}

// find returns true if buf, which holds a sequence of ZNG value messages, might
// contain a record with a field whose fully-qualified name (e.g., a.b.c)
// matches the pattern. find also returns true if it encounters an error.
func (f *fieldNameFinder) find(zctx *resolver.Context, buf []byte) bool {
	f.checkedIDs.SetInt64(0)
	for len(buf) > 0 {
		code := buf[0]
		if code > zng.CtrlValueEscape {
			// Control messages are not expected.
			return true
		}
		var id int
		if code == zng.CtrlValueEscape {
			v, n := binary.Uvarint(buf[1:])
			if n <= 0 {
				return true
			}
			id = int(v)
			buf = buf[1+n:]
		} else {
			id = int(code)
			buf = buf[1:]
		}
		length, n := binary.Uvarint(buf)
		if n <= 0 {
			return true
		}
		buf = buf[n+int(length):]
		if f.checkedIDs.Bit(id) == 1 {
			continue
		}
		f.checkedIDs.SetBit(&f.checkedIDs, id, 1)
		t, err := zctx.LookupType(id)
		if err != nil {
			return true
		}
		tr, ok := zng.AliasedType(t).(*zng.TypeRecord)
		if !ok {
			return true
		}
		for f.fieldNameIter.init(tr); !f.fieldNameIter.done(); {
			name := f.fieldNameIter.next()
			if f.stringCaseFinder.next(byteconv.UnsafeString(name)) != -1 {
				return true
			}
		}
	}
	return false
}

type fieldNameIter struct {
	buf   []byte
	stack []fieldNameIterInfo
}

type fieldNameIterInfo struct {
	columns []zng.Column
	offset  int
}

func (f *fieldNameIter) init(t *zng.TypeRecord) {
	f.buf = f.buf[:0]
	f.stack = f.stack[:0]
	if len(t.Columns) > 0 {
		f.stack = append(f.stack, fieldNameIterInfo{t.Columns, 0})
	}
}

func (f *fieldNameIter) done() bool {
	return len(f.stack) == 0
}

func (f *fieldNameIter) next() []byte {
	// Step into non-empty records.
	for {
		info := &f.stack[len(f.stack)-1]
		col := info.columns[info.offset]
		f.buf = append(f.buf, "."+col.Name...)
		t, ok := zng.AliasedType(col.Type).(*zng.TypeRecord)
		if !ok || len(t.Columns) == 0 {
			break
		}
		f.stack = append(f.stack, fieldNameIterInfo{t.Columns, 0})
	}
	// Skip leading dot.
	name := f.buf[1:]
	// Advance our position and step out of records.
	for len(f.stack) > 0 {
		info := &f.stack[len(f.stack)-1]
		col := info.columns[info.offset]
		f.buf = f.buf[:len(f.buf)-len(col.Name)-1]
		info.offset++
		if info.offset < len(info.columns) {
			break
		}
		f.stack = f.stack[:len(f.stack)-1]
	}
	return name
}
