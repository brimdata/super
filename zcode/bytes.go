// Package zcode implements serialization and deserialzation for ZNG values.
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
	"errors"
)

var (
	ErrNotContainer = errors.New("not a container")
	ErrNotSingleton = errors.New("not a single container")
)

// Bytes is the serialized representation of a sequence of ZNG values.
type Bytes []byte

// Iter returns an Iter for the receiver.
func (b Bytes) Iter() Iter {
	return Iter(b)
}

func (b Bytes) Body() (Bytes, error) {
	it := b.Iter()
	body := it.Next()
	if !it.Done() {
		return nil, ErrNotSingleton
	}
	return body, nil
}

// Append appends val to dst as a tagged value and returns the
// extended buffer.
func Append(dst Bytes, val []byte) Bytes {
	if val == nil {
		return AppendUvarint(dst, tagNull)
	}
	dst = AppendUvarint(dst, toTag(len(val)))
	return append(dst, val...)
}

// AppendUvarint is like encoding/binary.PutUvarint but appends to dst instead
// of writing into it.
func AppendUvarint(dst []byte, u64 uint64) []byte {
	for u64 >= 0x80 {
		dst = append(dst, byte(u64)|0x80)
		u64 >>= 7
	}
	return append(dst, byte(u64))
}

// sizeOfUvarint returns the number of bytes required by appendUvarint to
// represent u64.
func sizeOfUvarint(u64 uint64) int {
	n := 1
	for u64 >= 0x80 {
		n++
		u64 >>= 7
	}
	return n
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
