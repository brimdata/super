package zcode

import (
	"math"
)

// These functions provide variable length encoding and decoding of
// signed and unsigned integers when the length of the buffer is known,
// e.g., when the encoding lies within the body of a ZNG counted-length
// value.

func DecodeCountedUvarint(b []byte) uint64 {
	n := len(b)
	u64 := uint64(0)
	for n > 0 {
		n--
		u64 <<= 8
		u64 |= uint64(b[n])
	}
	return u64
}

func EncodeCountedUvarint(dst []byte, u64 uint64) uint {
	var n uint
	for u64 != 0 {
		dst[n] = byte(u64)
		u64 >>= 8
		n++
	}
	return n
}

func DecodeCountedVarint(b []byte) int64 {
	u64 := DecodeCountedUvarint(b)
	if u64&1 != 0 {
		u64 >>= 1
		if u64 == 0 {
			return math.MinInt64
		}
		return -int64(u64)
	}
	return int64(u64 >> 1)
}

func EncodeCountedVarint(dst []byte, i int64) uint {
	var u64 uint64
	if i >= 0 {
		u64 = uint64(i) << 1
	} else {
		u64 = uint64(-i)<<1 | 1
	}
	return EncodeCountedUvarint(dst, u64)
}
