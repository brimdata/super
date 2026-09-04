package expr

import (
	"strings"

	"github.com/brimdata/super"
)

// StringContainsFold is like strings.Contains but with case-insensitive
// comparison.
func StringContainsFold(a, b string) bool {
	alen := len(a)
	blen := len(b)

	if blen > alen {
		return false
	}

	end := alen - blen + 1
	i := 0
	for i < end {
		if strings.EqualFold(a[i:i+blen], b) {
			return true
		}
		i++
	}
	return false
}

func IsTrue(val super.Value) bool {
	return val.Deunion().Ptr().AsBool()
}
