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
	checkedIDs    big.Int
	fieldNameIter FieldNameIter
	caseFinder    *stringsearch.CaseFinder
}

func NewFieldNameFinder(pattern string) *FieldNameFinder {
	return &FieldNameFinder{caseFinder: stringsearch.NewCaseFinder(pattern)}
}

// Find returns true if buf, which holds a sequence of ZNG value messages, might
// contain a record with a field whose fully-qualified name (e.g., a.b.c)
// matches the pattern.  Find also returns true if it encounters an error.
func (f *FieldNameFinder) Find(zctx *super.Context, buf []byte) bool {
	f.checkedIDs.SetInt64(0)
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
		t, err := zctx.LookupType(int(id))
		if err != nil {
			return true
		}
		tr, ok := super.TypeUnder(t).(*super.TypeRecord)
		if !ok {
			return true
		}
		for f.fieldNameIter.Init(tr); !f.fieldNameIter.Done(); {
			name := f.fieldNameIter.Next()
			if f.caseFinder.Next(byteconv.UnsafeString(name)) != -1 {
				return true
			}
		}
	}
	return false
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
