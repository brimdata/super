package super

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/bits"
	"net/netip"

	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/zcode"
	"github.com/x448/float16"
)

type TypeOfBool struct{}

func AppendBool(zb zcode.Bytes, b bool) zcode.Bytes {
	if b {
		return append(zb, 1)
	}
	return append(zb, 0)
}

func EncodeBool(b bool) zcode.Bytes {
	return AppendBool(nil, b)
}

func DecodeBool(zv zcode.Bytes) bool {
	return zv != nil && zv[0] != 0
}

func (t *TypeOfBool) ID() int {
	return IDBool
}

func (t *TypeOfBool) Kind() Kind {
	return PrimitiveKind
}

type TypeOfBytes struct{}

func EncodeBytes(b []byte) zcode.Bytes {
	return zcode.Bytes(b)
}

func DecodeBytes(zv zcode.Bytes) []byte {
	return []byte(zv)
}

func (t *TypeOfBytes) ID() int {
	return IDBytes
}

func (t *TypeOfBytes) Kind() Kind {
	return PrimitiveKind
}

func (t *TypeOfBytes) Format(zv zcode.Bytes) string {
	return "0x" + hex.EncodeToString(zv)
}

type TypeOfDuration struct{}

func EncodeDuration(d nano.Duration) zcode.Bytes {
	return EncodeInt(int64(d))
}

func AppendDuration(bytes zcode.Bytes, d nano.Duration) zcode.Bytes {
	return AppendInt(bytes, int64(d))
}

func DecodeDuration(zv zcode.Bytes) nano.Duration {
	return nano.Duration(DecodeInt(zv))
}

func (t *TypeOfDuration) ID() int {
	return IDDuration
}

func (t *TypeOfDuration) Kind() Kind {
	return PrimitiveKind
}

func DecodeFloat(zb zcode.Bytes) float64 {
	if zb == nil {
		return 0
	}
	switch len(zb) {
	case 2:
		bits := binary.LittleEndian.Uint16(zb)
		return float64(float16.Frombits(bits).Float32())
	case 4:
		bits := binary.LittleEndian.Uint32(zb)
		return float64(math.Float32frombits(bits))
	case 8:
		bits := binary.LittleEndian.Uint64(zb)
		return math.Float64frombits(bits)
	}
	panic("float encoding is neither 4 nor 8 bytes")
}

type TypeOfFloat16 struct{}

func AppendFloat16(zb zcode.Bytes, f float32) zcode.Bytes {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, float16.Fromfloat32(f).Bits())
	return append(zb, buf...)
}

func EncodeFloat16(d float32) zcode.Bytes {
	var b [2]byte
	return AppendFloat16(b[:0], d)
}

func DecodeFloat16(zb zcode.Bytes) float32 {
	if zb == nil {
		return 0
	}
	return float16.Frombits(binary.LittleEndian.Uint16(zb)).Float32()
}

func (t *TypeOfFloat16) ID() int {
	return IDFloat16
}

func (t *TypeOfFloat16) Kind() Kind {
	return PrimitiveKind
}

type TypeOfFloat32 struct{}

func AppendFloat32(zb zcode.Bytes, f float32) zcode.Bytes {
	return binary.LittleEndian.AppendUint32(zb, math.Float32bits(f))
}

func EncodeFloat32(d float32) zcode.Bytes {
	var b [4]byte
	return AppendFloat32(b[:0], d)
}

func DecodeFloat32(zb zcode.Bytes) float32 {
	if zb == nil {
		return 0
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(zb))
}

func (t *TypeOfFloat32) ID() int {
	return IDFloat32
}

func (t *TypeOfFloat32) Kind() Kind {
	return PrimitiveKind
}

type TypeOfFloat64 struct{}

func AppendFloat64(zb zcode.Bytes, d float64) zcode.Bytes {
	return binary.LittleEndian.AppendUint64(zb, math.Float64bits(d))
}

func EncodeFloat64(d float64) zcode.Bytes {
	var b [8]byte
	return AppendFloat64(b[:0], d)
}

func DecodeFloat64(zv zcode.Bytes) float64 {
	if zv == nil {
		return 0
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(zv))
}

func (t *TypeOfFloat64) ID() int {
	return IDFloat64
}

func (t *TypeOfFloat64) Kind() Kind {
	return PrimitiveKind
}

func EncodeInt(i int64) zcode.Bytes {
	var b [8]byte
	n := zcode.EncodeCountedVarint(b[:], i)
	return b[:n]
}

func AppendInt(bytes zcode.Bytes, i int64) zcode.Bytes {
	return zcode.AppendCountedVarint(bytes, i)
}

func EncodeUint(i uint64) zcode.Bytes {
	var b [8]byte
	n := zcode.EncodeCountedUvarint(b[:], i)
	return b[:n]
}

func AppendUint(bytes zcode.Bytes, i uint64) zcode.Bytes {
	return zcode.AppendCountedUvarint(bytes, i)
}

func DecodeInt(zv zcode.Bytes) int64 {
	return zcode.DecodeCountedVarint(zv)
}

func DecodeUint(zv zcode.Bytes) uint64 {
	return zcode.DecodeCountedUvarint(zv)
}

type TypeOfInt8 struct{}

func (t *TypeOfInt8) ID() int {
	return IDInt8
}

func (t *TypeOfInt8) Kind() Kind {
	return PrimitiveKind
}

type TypeOfUint8 struct{}

func (t *TypeOfUint8) ID() int {
	return IDUint8
}

func (t *TypeOfUint8) Kind() Kind {
	return PrimitiveKind
}

type TypeOfInt16 struct{}

func (t *TypeOfInt16) ID() int {
	return IDInt16
}

func (t *TypeOfInt16) Kind() Kind {
	return PrimitiveKind
}

type TypeOfUint16 struct{}

func (t *TypeOfUint16) ID() int {
	return IDUint16
}

func (t *TypeOfUint16) Kind() Kind {
	return PrimitiveKind
}

type TypeOfInt32 struct{}

func (t *TypeOfInt32) ID() int {
	return IDInt32
}

func (t *TypeOfInt32) Kind() Kind {
	return PrimitiveKind
}

type TypeOfUint32 struct{}

func (t *TypeOfUint32) ID() int {
	return IDUint32
}

func (t *TypeOfUint32) Kind() Kind {
	return PrimitiveKind
}

type TypeOfInt64 struct{}

func (t *TypeOfInt64) ID() int {
	return IDInt64
}

func (t *TypeOfInt64) Kind() Kind {
	return PrimitiveKind
}

type TypeOfUint64 struct{}

func (t *TypeOfUint64) ID() int {
	return IDUint64
}

func (t *TypeOfUint64) Kind() Kind {
	return PrimitiveKind
}

type TypeOfIP struct{}

func AppendIP(zb zcode.Bytes, a netip.Addr) zcode.Bytes {
	return append(zb, a.AsSlice()...)
}

func EncodeIP(a netip.Addr) zcode.Bytes {
	return AppendIP(nil, a)
}

func DecodeIP(zv zcode.Bytes) netip.Addr {
	var a netip.Addr
	if err := a.UnmarshalBinary(zv); err != nil {
		panic(fmt.Errorf("failure trying to decode IP address: %w", err))
	}
	return a
}

func (t *TypeOfIP) ID() int {
	return IDIP
}

func (t *TypeOfIP) Kind() Kind {
	return PrimitiveKind
}

type TypeOfNet struct{}

var ones = [16]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

func AppendNet(zb zcode.Bytes, p netip.Prefix) zcode.Bytes {
	// Mask for canonical form.
	p = p.Masked()
	zb = append(zb, p.Addr().AsSlice()...)
	length := p.Addr().BitLen() / 8
	onesAddr, ok := netip.AddrFromSlice(ones[:length])
	if !ok {
		panic(fmt.Sprintf("bad slice length %d for %s", length, p))
	}
	mask := netip.PrefixFrom(onesAddr, p.Bits()).Masked()
	return append(zb, mask.Addr().AsSlice()...)
}

func EncodeNet(p netip.Prefix) zcode.Bytes {
	return AppendNet(nil, p)
}

func DecodeNet(zv zcode.Bytes) netip.Prefix {
	if zv == nil {
		return netip.Prefix{}
	}
	a, ok := netip.AddrFromSlice(zv[:len(zv)/2])
	if !ok {
		panic("failure trying to decode IP subnet that is not 8 or 32 bytes long")
	}
	return netip.PrefixFrom(a, LeadingOnes(zv[len(zv)/2:]))
}

// LeadingOnes returns the number of leading one bits in b.
func LeadingOnes(b []byte) int {
	var n int
	for ; len(b) > 0; b = b[1:] {
		n += bits.LeadingZeros8(b[0] ^ 0xff)
		if b[0] != 0xff {
			break
		}
	}
	return n
}

func (t *TypeOfNet) ID() int {
	return IDNet
}

func (t *TypeOfNet) Kind() Kind {
	return PrimitiveKind
}

type TypeOfNull struct{}

func (t *TypeOfNull) ID() int {
	return IDNull
}

func (t *TypeOfNull) Kind() Kind {
	return PrimitiveKind
}

type TypeOfString struct{}

func EncodeString(s string) zcode.Bytes {
	return zcode.Bytes(s)
}

func DecodeString(zv zcode.Bytes) string {
	return string(zv)
}

func (t *TypeOfString) ID() int {
	return IDString
}

func (t *TypeOfString) Kind() Kind {
	return PrimitiveKind
}

type TypeOfTime struct{}

func EncodeTime(t nano.Ts) zcode.Bytes {
	var b [8]byte
	n := zcode.EncodeCountedVarint(b[:], int64(t))
	return b[:n]
}

func AppendTime(bytes zcode.Bytes, t nano.Ts) zcode.Bytes {
	return AppendInt(bytes, int64(t))
}

func DecodeTime(zv zcode.Bytes) nano.Ts {
	return nano.Ts(zcode.DecodeCountedVarint(zv))
}

func (t *TypeOfTime) ID() int {
	return IDTime
}

func (t *TypeOfTime) Kind() Kind {
	return PrimitiveKind
}
