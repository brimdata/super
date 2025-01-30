// Package byteconv implements conversions from byte slice representations of
// various data types.
package byteconv

import (
	"net/netip"
	"strconv"
	"unsafe"
)

// UnsafeString converts a byte slice to a string without copying the underlying
// data.  Unsafe string coversion is OK when calling some other function that
// doesn't store the string and otherwise never uses the string again.
func UnsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func ParseBool(b []byte) (bool, error) {
	return strconv.ParseBool(UnsafeString(b))
}

func ParseIP(b []byte) (netip.Addr, error) {
	return netip.ParseAddr(UnsafeString(b))
}

func ParseInt8(b []byte) (int8, error) {
	v, err := strconv.ParseInt(UnsafeString(b), 10, 8)
	return int8(v), err
}

func ParseUint8(b []byte) (uint8, error) {
	v, err := strconv.ParseUint(UnsafeString(b), 10, 8)
	return uint8(v), err
}

func ParseInt16(b []byte) (int16, error) {
	v, err := strconv.ParseInt(UnsafeString(b), 10, 16)
	return int16(v), err
}

func ParseUint16(b []byte) (uint16, error) {
	v, err := strconv.ParseUint(UnsafeString(b), 10, 16)
	return uint16(v), err
}

func ParseInt32(b []byte) (int32, error) {
	v, err := strconv.ParseInt(UnsafeString(b), 10, 32)
	return int32(v), err
}

func ParseUint32(b []byte) (uint32, error) {
	v, err := strconv.ParseUint(UnsafeString(b), 10, 32)
	return uint32(v), err
}

func ParseInt64(b []byte) (int64, error) {
	return strconv.ParseInt(UnsafeString(b), 10, 64)
}

func ParseUint64(b []byte) (uint64, error) {
	return strconv.ParseUint(UnsafeString(b), 10, 64)
}

func ParseFloat64(b []byte) (float64, error) {
	return strconv.ParseFloat(UnsafeString(b), 64)
}

func ReinterpretSlice[Out, In any](in []In) []Out {
	outData := (*Out)(unsafe.Pointer(unsafe.SliceData(in)))
	outLen := len(in) * int(unsafe.Sizeof(in[0])) / int(unsafe.Sizeof(*outData))
	outCap := cap(in) * int(unsafe.Sizeof(in[0])) / int(unsafe.Sizeof(*outData))
	return unsafe.Slice(outData, outCap)[:outLen]
}
