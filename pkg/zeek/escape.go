package zeek

import (
	"bytes"
)

// Escape returns a representation of data with \ replaced by \\ and with all
// bytes outside the range from 0x20 through 0x7e replaced by a \xhh sequence.
// This is string escaping scheme implemented by Zeek's ASCII log writer.
func Escape(data []byte) string {
	var buf []byte
	for _, c := range data {
		switch {
		case c == '\\':
			buf = append(buf, c, c)
		case c < 0x20 || 0x7e < c:
			const hexdigits = "0123456789abcdef"
			buf = append(buf, '\\', 'x', hexdigits[c>>4], hexdigits[c&0xf])
		default:
			buf = append(buf, c)
		}
	}
	return string(buf)
}

// Unescape is the inverse of Escape.
func Unescape(data []byte) []byte {
	if bytes.IndexByte(data, '\\') < 0 {
		return data
	}
	var buf []byte
	i := 0
	for i < len(data) {
		c := data[i]
		if c == '\\' && len(data[i:]) >= 2 {
			var n int
			c, n = ParseEscape(data[i:])
			i += n
		} else {
			i++
		}
		buf = append(buf, c)
	}
	return buf
}

func ParseEscape(data []byte) (byte, int) {
	if len(data) >= 4 && data[1] == 'x' {
		v1 := unhex(data[2])
		v2 := unhex(data[3])
		if v1 <= 0xf || v2 <= 0xf {
			return v1<<4 | v2, 4
		}
	}
	return data[1], 2
}

func unhex(b byte) byte {
	switch {
	case '0' <= b && b <= '9':
		return b - '0'
	case 'a' <= b && b <= 'f':
		return b - 'a' + 10
	case 'A' <= b && b <= 'F':
		return b - 'A' + 10
	}
	return 255
}
