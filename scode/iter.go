package scode

import (
	"encoding/binary"
	"fmt"
)

// Iter iterates over the sequence of values encoded in Bytes.
type Iter Bytes

// Done returns true if no values remain.
func (i *Iter) Done() bool {
	return len(*i) == 0
}

// Next returns the body of the next value along with a boolean that is true if
// the value is a container.  It returns an empty slice for an empty or
// zero-length value and nil for a Zed null value.  The container boolean is not
// meaningful if the returned Bytes slice is nil.  Next panics if the next value
// is malformed.
func (i *Iter) Next() Bytes {
	// The tag is zero for a null value; otherwise, it is the value's
	// length plus one.
	u64, n := binary.Uvarint(*i)
	if n <= 0 {
		panic(fmt.Sprintf("bad uvarint: %d", n))
	}
	if tagIsNull(u64) {
		*i = (*i)[n:]
		return nil
	}
	end := n + tagLength(u64)
	val := (*i)[n:end]
	*i = (*i)[end:]
	return Bytes(val)
}

// NextTagAndBody returns the next value as a slice containing the value's
// undecoded tag followed by its body.  NextTagAndBody panics if the next
// value is malformed.
func (i *Iter) NextTagAndBody() Bytes {
	u64, n := binary.Uvarint(*i)
	if n <= 0 {
		panic(fmt.Sprintf("bad uvarint: %d", n))
	}
	if !tagIsNull(u64) {
		n += tagLength(u64)
	}
	val := (*i)[:n]
	*i = (*i)[n:]
	return Bytes(val)
}
