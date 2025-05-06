package bitvec

import (
	"math/bits"
	"strings"
)

type Bits struct {
	bits   []uint64
	length uint32
}

var Zero Bits

func New(bits []uint64, length uint32) Bits {
	return Bits{length: length, bits: bits}
}

func NewFalse(length uint32) Bits {
	return Bits{length: length, bits: make([]uint64, (length+63)/64)}
}

func NewTrue(n uint32) Bits {
	b := NewFalse(n)
	for i := range b.bits {
		b.bits[i] = ^uint64(0)
	}
	return b
}

func (b Bits) IsZero() bool {
	return b.length == 0
}

// GetBits returns b's underlying storage with used bits cleared.
// GetBits may modify the underlying storage.
func (b Bits) GetBits() []uint64 {
	if unusedBits := 64 - (b.length % 64); unusedBits < 64 {
		// Clear unused bits.
		mask := ^uint64(0) >> unusedBits
		b.bits[len(b.bits)-1] &= mask
	}
	return b.bits
}

func (b Bits) IsSet(slot uint32) bool {
	// Because Bits is used to store nulls for many vectors and it is often
	// nil check to see if receiver is nil and return false.
	return !b.IsZero() && b.IsSetDirect(slot)
}

func (b Bits) IsSetDirect(slot uint32) bool {
	return (b.bits[slot>>6] & (1 << (slot & 0x3f))) != 0
}

// Set causes the bit at position slot to become true on an allocated
// bitvector, where slot must be smaller than the length of the bit vector.
func (b Bits) Set(slot uint32) {
	b.bits[slot>>6] |= (1 << (slot & 0x3f))
}

// Shorten may be called to shorten the length of an allocated vector.
// This is useful when you know that a vector has a limit but you're not
// sure how large it might be.  Create the vector with max length, write
// to it with Set, then shorten it to the actual length.
func (b *Bits) Shorten(length uint32) {
	b.length = length
}

func (b *Bits) Len() uint32 {
	if b == nil { //XXX do we need nil check?
		return 0
	}
	return b.length
}

func (b Bits) Pick(index []uint32) Bits {
	if b.IsZero() || len(index) == 0 {
		return Zero
	}
	out := NewFalse(uint32(len(index)))
	for k, slot := range index {
		if b.IsSetDirect(slot) {
			out.Set(uint32(k))
		}
	}
	return out
}

func (b Bits) ReversePick(index []uint32) Bits {
	return b.Pick(ReverseIndex(index, b.length))
}

func (b Bits) TrueCount() uint32 {
	if b.IsZero() {
		return 0
	}
	var n uint32
	for _, bs := range b.bits {
		n += uint32(bits.OnesCount64(bs))
	}
	if numTailBits := b.Len() % 64; numTailBits > 0 {
		mask := ^uint64(0) << numTailBits
		unusedBits := b.bits[len(b.bits)-1] & mask
		n -= uint32(bits.OnesCount64(unusedBits))
	}
	return n
}

// helpful to have around for debugging
func (b Bits) String() string {
	if b.IsZero() {
		return "empty"
	}
	var s strings.Builder
	for k := range b.Len() {
		if b.IsSet(k) {
			s.WriteByte('1')
		} else {
			s.WriteByte('0')
		}
	}
	return s.String()
}

func Not(a Bits) Bits {
	not := NewFalse(a.Len())
	for i := range a.bits {
		not.bits[i] = ^a.bits[i]
	}
	return not
}

func Or(a, b Bits) Bits {
	if b.IsZero() {
		return a
	}
	if a.IsZero() {
		return b
	}
	if a.Len() != b.Len() {
		panic("or'ing two different length bool vectors")
	}
	out := NewFalse(a.Len())
	for i := range len(a.bits) {
		out.bits[i] = a.bits[i] | b.bits[i]
	}
	return out
}

func And(a, b Bits) Bits {
	if a.IsZero() || b.IsZero() {
		return Zero
	}
	if a.Len() != b.Len() {
		panic("and'ing two different length bool vectors")
	}
	out := NewFalse(a.Len())
	for i := range len(a.bits) {
		out.bits[i] = a.bits[i] & b.bits[i]
	}
	return out
}

func ReverseIndex(index []uint32, n uint32) []uint32 {
	var reverse []uint32
	for i := range n {
		if len(index) > 0 && index[0] == i {
			index = index[1:]
			continue
		}
		reverse = append(reverse, i)
	}
	return reverse
}
