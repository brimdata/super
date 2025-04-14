package zeekio

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/nano"
)

// formatTime formats ts as a Zeek time value.  A Zeek time value is a
// floating-point number representing seconds since (or before, for negative
// values) the Unix epoch.  Since a float64 lacks sufficient precision to
// represent an arbitrary nano.Ts value, formatTime and parseTime do extra work
// to preserve precision that would be lost by using strconv.FormatFloat and
// ParseFloat.
func formatTime(ts nano.Ts) string {
	sec, ns := ts.Split()
	// Zeek uses a precision of 6.  See
	// https://github.com/zeek/zeek/blob/v4.2.0/src/threading/formatters/Ascii.cc#L114-L119
	// and
	// https://github.com/zeek/zeek/blob/v4.2.0/src/threading/Formatter.cc#L109-L112.
	precision := 6
	if (ns/1000)*1000 != ns {
		// Increase precision to prevent rounding.
		precision = 9
	}
	var negative bool
	if sec < 0 {
		sec = sec * -1
		negative = true
	}
	if ns < 0 {
		ns = ns * -1
		negative = true
	}
	var dst []byte
	if negative {
		dst = append(dst, '-')
	}
	dst = strconv.AppendInt(dst, sec, 10)
	if ns > 0 || precision > 0 {
		n := len(dst)
		dst = strconv.AppendFloat(dst, float64(ns)/1e9, 'f', precision, 64)
		// Remove the first '0'.  This is a little hacky but the alternative
		// is implementing this ourselves.  Something to avoid given
		// https://golang.org/src/math/big/ftoa.go?s=2522:2583#L53.
		dst = slices.Delete(dst, n, n+1)
	}
	return string(dst)
}

// parseTime interprets b as a Zeek time value and returns the corresponding
// value.  See formatTime for details.
func parseTime(b []byte) (nano.Ts, error) {
	if ts, ok := parseTimeDecimal(b); ok {
		return ts, nil
	}
	// Slow path for scientific notation.
	if f, err := byteconv.ParseFloat64(b); err == nil {
		sec := math.Round(f)
		ns := f - sec
		return nano.Ts(int64(sec)*1e9 + int64(ns*1e9)), nil
	}
	return 0, fmt.Errorf("invalid time format: %q", b)
}

func parseTimeDecimal(b []byte) (nano.Ts, bool) {
	var v, scale, sign int64
	sign = 1
	scale = 1000000000
	k := 0
	n := len(b)
	if n == 0 {
		return 0, false
	}
	if b[0] == '-' {
		if n == 1 {
			return 0, false
		}
		sign, k = -1, 1
	}
	for ; k < n; k++ {
		c := b[k]
		if c != '.' && (c < '0' || c > '9') {
			return 0, false
		}
		if c == '.' {
			for k++; k < n; k++ {
				c = b[k]
				if c < '0' || c > '9' {
					return 0, false
				}
				v = v*10 + int64(c-'0')
				scale /= 10
			}
			break
		}
		v = v*10 + int64(c-'0')
	}
	return nano.Ts(sign * v * scale), true
}
