package zeekio

import (
	"testing"
	"time"

	"github.com/brimdata/super/pkg/nano"
	"github.com/stretchr/testify/assert"
)

func TestFormatTime(t *testing.T) {
	assert.Equal(t, "60.000000001", formatTime(nano.Ts(time.Minute+1)))
	assert.Equal(t, "-60.000000010", formatTime(-nano.Ts(time.Minute+10)))
	assert.Equal(t, "-60.000000", formatTime(-nano.Ts(time.Minute)))
	assert.Equal(t, "-0.100000", formatTime(-nano.Ts(time.Millisecond*100)))
}

func TestParseTime(t *testing.T) {
	cases := []struct {
		input      string
		expectedTs nano.Ts
	}{
		{"0", 0},
		{"1425565514.419939", 1425565514419939000},
		{"001425565514.419939", 1425565514419939000},
		{"-1425565514.419939", -1425565514419939000},
		{"1e9", 1e9 * 1e9},
		{"1.123e8", 1.123e8 * 1e9},
		{"1.123e-5", nano.Ts(1.123e-5 * 1e9)},
	}
	for _, c := range cases {
		ts, err := parseTime([]byte(c.input))
		assert.NoError(t, err, "input: %q", c.input)
		assert.Exactly(t, c.expectedTs, ts, "input: %q", c.input)
	}
	for _, input := range []string{"", " ", "a"} {
		_, err := parseTime([]byte(input))
		assert.Error(t, err, "input: %q", input)
	}
}
