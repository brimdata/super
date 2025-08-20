package zeekio

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnescapeZeekString(t *testing.T) {
	cases := []struct {
		unescaped string
		escaped   string
	}{
		{`\`, `\\`},
		{`\\`, `\\\\`},
		{`ascii`, `ascii`},
		{"\a\b\f\n\r\t\v", `\x07\x08\x0c\x0a\x0d\x09\x0b`},
		{"\x00\x19\x20\\\x7e\x7f\xff", "\\x00\\x19\x20\\\\\x7e\\x7f\\xff"},
		{"\x00😁", `\x00\xf0\x9f\x98\x81`},
	}
	for _, c := range cases {
		in, expected := c.escaped, c.unescaped

		actual := unescapeZeekString([]byte(in))
		require.Exactly(t, []byte(expected), actual, "case: %#v", c)

		actual = unescapeZeekString([]byte("prefix" + in + "suffix"))
		expected = "prefix" + expected + "suffix"
		require.Exactly(t, []byte(expected), actual, "case: %#v", c)
	}
}
