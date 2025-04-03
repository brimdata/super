package zeekio

import (
	"bytes"
	"unicode"
	"unicode/utf8"

	"github.com/brimdata/super/sup"
)

// shouldEscape determines if the given code point at the given position
// in a value should be escaped for the given output format.  This function
// does not account for unprintable characters, its main purpose is to
// centralize the logic about which characters are syntatically significant
// in each output format and hence must be escaped.  The inContainer parameter
// specifies whether this values is inside a set or vector (which is needed
// to correctly implement Zeek log escaping rules).
func shouldEscape(r rune, inContainer bool) bool {
	switch r {
	case '\\', '\t':
		return true
	case ',':
		return inContainer
	default:
		return false
	}
}

const hexdigits = "0123456789abcdef"

func EscapeZeekHex(b []byte) []byte {
	var out []byte
	var start int
	for i := 0; i < len(b); {
		r, l := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError || !unicode.IsPrint(r) {
			out = append(out, b[start:i]...)
			// XXX format l chars
			c := b[i]
			out = append(out, '\\', 'x', hexdigits[c>>4], hexdigits[c&0xf])
			i++
			start = i
		} else {
			i += l
		}
	}
	return append(out, b[start:]...)
}

func unescapeZeekString(data []byte) []byte {
	if bytes.IndexByte(data, '\\') < 0 {
		return data
	}
	var buf []byte
	i := 0
	for i < len(data) {
		c := data[i]
		if c == '\\' && len(data[i:]) >= 2 {
			var n int
			c, n = parseZeekEscape(data[i:])
			i += n
		} else {
			i++
		}
		buf = append(buf, c)
	}
	return buf
}

func parseZeekEscape(data []byte) (byte, int) {
	if len(data) >= 4 && data[1] == 'x' {
		v1 := sup.Unhex(data[2])
		v2 := sup.Unhex(data[3])
		if v1 <= 0xf || v2 <= 0xf {
			return v1<<4 | v2, 4
		}
	} else if len(data) >= 2 {
		if c := unesc(data[1]); c != 0 {
			return c, 2
		}
	}
	// Not a valid escape sequence, just leave it alone.
	return data[0], 1
}

func unesc(c byte) byte {
	switch c {
	case '\\':
		return '\\'
	case '"':
		return '"'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	}
	return 0
}
