package expr

import (
	"unicode/utf8"
)

func FixSliceBounds(start, end, size int) (int, int) {
	if start > end || end < 0 {
		return 0, 0
	}
	return max(start, 0), min(end, size)
}

// UTF8PrefixLen returns the length in bytes of the first runeCount runes in b.
// It returns 0 if runeCount<0 and len(b) if runeCount>utf8.RuneCount(b).
func UTF8PrefixLen(b []byte, runeCount int) int {
	var i, runeCurrent int
	for {
		if runeCurrent >= runeCount {
			return i
		}
		r, n := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && n == 0 {
			return i
		}
		i += n
		runeCurrent++
	}
}
