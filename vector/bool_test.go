package vector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBoolTrueCount(t *testing.T) {
	assert.EqualValues(t, 0, (*Bool)(nil).TrueCount())
	bits := []uint64{0xff}
	assert.EqualValues(t, 1, NewBool(bits, 1, nil).TrueCount())
	assert.EqualValues(t, 8, NewBool(bits, 64, nil).TrueCount())
	bits = []uint64{0xff, 0xff << 56}
	assert.EqualValues(t, 8, NewBool(bits, 65, nil).TrueCount())
	assert.EqualValues(t, 15, NewBool(bits, 127, nil).TrueCount())
	assert.EqualValues(t, 16, NewBool(bits, 128, nil).TrueCount())
}
