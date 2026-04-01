package jsupio

import (
	"strings"
	"testing"

	"github.com/brimdata/super"
	"github.com/stretchr/testify/require"
)

func TestDecodeEnumRejectsNegativeIndex(t *testing.T) {
	input := `{"type":{"kind":"enum","id":30,"symbols":["ok"]},"value":"-1"}` + "\n"
	r := NewReader(super.NewContext(), strings.NewReader(input))

	val, err := r.Read()
	require.Nil(t, val)
	require.EqualError(t, err, "line 1: JSUP enum index value is negative")
}

func TestDecodeEnumRejectsOutOfRangeIndex(t *testing.T) {
	input := `{"type":{"kind":"enum","id":30,"symbols":["ok"]},"value":"1"}` + "\n"
	r := NewReader(super.NewContext(), strings.NewReader(input))

	val, err := r.Read()
	require.Nil(t, val)
	require.EqualError(t, err, "line 1: JSUP enum index value out of range")
}
