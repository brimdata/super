package bitvec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitsGetBits(t *testing.T) {
	assert.Equal(t, []uint64{0x0f}, New([]uint64{0xff}, 4).GetBits())
}

func TestBitsTrueCount(t *testing.T) {
	assert.EqualValues(t, 0, Zero.TrueCount())
	bits := []uint64{0xff}
	assert.EqualValues(t, 1, New(bits, 1).TrueCount())
	assert.EqualValues(t, 8, New(bits, 64).TrueCount())
	bits = []uint64{0xff, 0xff << 56}
	assert.EqualValues(t, 8, New(bits, 65).TrueCount())
	assert.EqualValues(t, 15, New(bits, 127).TrueCount())
	assert.EqualValues(t, 16, New(bits, 128).TrueCount())
}
