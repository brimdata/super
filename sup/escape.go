package sup

import (
	"strings"
	"unicode/utf8"
)

func QuotedName(name string) string {
	if !IsIdentifier(name) {
		name = QuotedString(name)
	}
	return name
}

func QuotedTypeName(name string) string {
	if !IsTypeName(name) {
		name = QuotedString(name)
	}
	return name
}

const hexdigits = "0123456789abcdef"

// QuotedString quotes and escapes a SUP string for serialization in accordance
// with the SUP spec.  It was copied and modified
// [with attribution](https://github.com/brimdata/super/blob/main/acknowledgments.txt)
// from the encoding/json package in the Go source code.
func QuotedString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for k := 0; k < len(s); {
		if c := s[k]; c < utf8.RuneSelf {
			if safeSet[c] {
				b.WriteByte(c)
				k++
				continue
			}
			b.WriteByte('\\')
			switch c {
			case '\\', '"':
				b.WriteByte(c)
			case '\b':
				b.WriteByte('b')
			case '\f':
				b.WriteByte('f')
			case '\n':
				b.WriteByte('n')
			case '\r':
				b.WriteByte('r')
			case '\t':
				b.WriteByte('t')
			default:
				// ASCII control codes other than above
				b.WriteString(`u00`)
				b.WriteByte(hexdigits[c>>4])
				b.WriteByte(hexdigits[c&0xF])
			}
			k++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[k:])
		if r == utf8.RuneError && size == 1 {
			// XXX return an error.  See issue #3455.
			b.WriteString(`\ufffd`)
			k += size
			continue
		}
		b.WriteRune(r)
		k += size
	}
	b.WriteByte('"')
	return b.String()
}

func Unhex(b byte) byte {
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

// safeSet holds the value true if the ASCII character with the given array
// position can be represented inside a JSON string without any further
// escaping.
//
// All values are true except for the ASCII control characters (0-31), the
// double quote ("), and the backslash character ("\").
//
// This code was copied [with attribution](https://github.com/brimdata/super/blob/main/acknowledgments.txt)
// from the encoding/json package in the Go source code.
var safeSet = [utf8.RuneSelf]bool{
	' ':      true,
	'!':      true,
	'"':      false,
	'#':      true,
	'$':      true,
	'%':      true,
	'&':      true,
	'\'':     true,
	'(':      true,
	')':      true,
	'*':      true,
	'+':      true,
	',':      true,
	'-':      true,
	'.':      true,
	'/':      true,
	'0':      true,
	'1':      true,
	'2':      true,
	'3':      true,
	'4':      true,
	'5':      true,
	'6':      true,
	'7':      true,
	'8':      true,
	'9':      true,
	':':      true,
	';':      true,
	'<':      true,
	'=':      true,
	'>':      true,
	'?':      true,
	'@':      true,
	'A':      true,
	'B':      true,
	'C':      true,
	'D':      true,
	'E':      true,
	'F':      true,
	'G':      true,
	'H':      true,
	'I':      true,
	'J':      true,
	'K':      true,
	'L':      true,
	'M':      true,
	'N':      true,
	'O':      true,
	'P':      true,
	'Q':      true,
	'R':      true,
	'S':      true,
	'T':      true,
	'U':      true,
	'V':      true,
	'W':      true,
	'X':      true,
	'Y':      true,
	'Z':      true,
	'[':      true,
	'\\':     false,
	']':      true,
	'^':      true,
	'_':      true,
	'`':      true,
	'a':      true,
	'b':      true,
	'c':      true,
	'd':      true,
	'e':      true,
	'f':      true,
	'g':      true,
	'h':      true,
	'i':      true,
	'j':      true,
	'k':      true,
	'l':      true,
	'm':      true,
	'n':      true,
	'o':      true,
	'p':      true,
	'q':      true,
	'r':      true,
	's':      true,
	't':      true,
	'u':      true,
	'v':      true,
	'w':      true,
	'x':      true,
	'y':      true,
	'z':      true,
	'{':      true,
	'|':      true,
	'}':      true,
	'~':      true,
	'\u007f': true,
}
