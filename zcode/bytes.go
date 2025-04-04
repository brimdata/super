// Package zcode implements serialization and deserialzation for BSUP values.
//
// Values of primitive type are represented by an unsigned integer tag and an
// optional byte-sequence body.  A tag of zero indicates that the value is
// null, and no body follows.  A nonzero tag indicates that the value is set,
// and the value itself follows as a body of length tag-1.
//
// Values of union type are represented similarly, with the body
// prefixed by an integer specifying the index determining the type of
// the value in reference to the union type.
//
// Values of container type (record, set, or array) are represented similarly,
// with the body containing a sequence of zero or more serialized values.
package zcode

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var ErrNotSingleton = errors.New("value body has more than one encoded value")

// Bytes is the serialized representation of a sequence of BSUP values.
type Bytes []byte

// Iter returns an Iter for the receiver.
func (b Bytes) Iter() Iter {
	return Iter(b)
}

// Body returns b's body.
func (b Bytes) Body() Bytes {
	it := b.Iter()
	return it.Next()
}

// Append appends val to dst as a tagged value and returns the
// extended buffer.
func Append(dst Bytes, val []byte) Bytes {
	if val == nil {
		return binary.AppendUvarint(dst, tagNull)
	}
	dst = binary.AppendUvarint(dst, toTag(len(val)))
	return append(dst, val...)
}

// SizeOfUvarint returns the number of bytes required by binary.AppendUvarint to
// represent u64.
func SizeOfUvarint(u64 uint64) int {
	n := 1
	for u64 >= 0x80 {
		n++
		u64 >>= 7
	}
	return n
}

func ReadTag(r io.ByteReader) (int, error) {
	// The tag is zero for a null value; otherwise, it is the value's
	// length plus one.
	u64, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, err
	}
	if tagIsNull(u64) {
		return -1, nil
	}
	return tagLength(u64), nil
}

func DecodeTagLength(b Bytes) int {
	u64, n := binary.Uvarint(b)
	if n <= 0 {
		panic(fmt.Sprintf("bad uvarint: %d", n))
	}
	if tagIsNull(u64) {
		return n
	}
	return int(u64) + n - 1
}

func toTag(length int) uint64 {
	return uint64(length) + 1
}

const tagNull = 0

func tagIsNull(t uint64) bool {
	return t == tagNull
}

func tagLength(t uint64) int {
	if t == tagNull {
		panic("tagLength called with null tag")
	}
	return int(t - 1)
}
