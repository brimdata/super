package nano_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/superdb/super/pkg/nano"
)

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in       string
		expected nano.Ts
	}{
		{`1234567890.1`, 1234567890},
		{`-1234567890.1`, -1234567890},
		{`"1234567890"`, 1234567890},
		{`"-1234567890"`, -1234567890},
		{`{"sec":1.1}`, 1 * 1e9},
		{`{"sec":1.1,"ns":234567890.1}`, 1234567890},
		{`{"sec":1.1,"ns":-234567890.1}`, 765432110},
		{`{"sec":-1.1}`, -1 * 1e9},
		{`{"sec":-1.1,"ns":234567890.1}`, -765432110},
		{`{"sec":-1.1,"ns":-234567890.1}`, -1234567890},
	}
	for _, c := range cases {
		var ts nano.Ts
		assert.NoError(t, ts.UnmarshalJSON([]byte(c.in)), "input: %q", c.in)
		assert.Equal(t, c.expected, ts, "input: %q", c.in)
	}

	var ts nano.Ts
	assert.EqualError(t, ts.UnmarshalJSON([]byte(`"1.1"`)), `invalid time format: "1.1"`)
	for _, s := range []string{`{}`, `{"ns":1}`} {
		assert.EqualError(t, ts.UnmarshalJSON([]byte(s)), "time object is not of the form {sec:x, ns:y}")
	}

}

func TestParseMillis(t *testing.T) {
	successCases := []struct {
		input      string
		expectedTs nano.Ts
	}{
		{"0", 0},
		{"00", 0},
		{"2", 2 * 1_000_000},
		{"03", 3 * 1_000_000},
		{"1234567890", 1234567890 * 1_000_000},
	}
	for _, c := range successCases {
		ts, err := nano.ParseMillis([]byte(c.input))
		assert.NoError(t, err, "input: %q", c.input)
		assert.Exactly(t, c.expectedTs, ts, "input: %q", c.input)
	}

	for _, input := range []string{"", " ", "+1", "-1", "a", "1.2", "1579438676648060000"} {
		_, err := nano.ParseMillis([]byte(input))
		assert.Error(t, err, "input: %q", input)
	}
}
