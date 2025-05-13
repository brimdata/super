// Package colw lays out columns for display of a list when you don't know
// ahead of time how many columns should exist.  Kind of like what ls does.
package colw

import (
	"errors"
	"io"
)

// ErrDoesNotFit is returned when the items do not fit in the supplied width.
// The only way this happens is if there is at least one item larger than the
// width less padding.  Otherwise, you would get one column taking up all the width.
var ErrDoesNotFit = errors.New("items do not fit")

func colwidth(in []string, pad int) (longest int) {
	for _, s := range in {
		longest = max(longest, len(s)+pad)
	}
	return
}

func makePadding(n int, padchar byte) []byte {
	b := make([]byte, n)
	for k := range n {
		b[k] = padchar
	}
	return b
}

var newline = []byte{'\n'}

// Write writes the list of strings to the given writer with padding and
// newlines according to Layout
func Write(w io.Writer, in []string, width, pad int) error {
	colw := colwidth(in, pad)
	if colw > width {
		return ErrDoesNotFit
	}
	ncol := width / colw
	padding := makePadding(width, ' ')
	for k, s := range in {
		if _, err := w.Write([]byte(s)); err != nil {
			return err
		}
		if k == len(in)-1 {
			_, err := w.Write(newline)
			return err
		}
		colno := k % ncol
		if colno == ncol-1 {
			if _, err := w.Write(newline); err != nil {
				return err
			}
		} else {
			pad := colw - len(s)
			if pad > 0 {
				if _, err := w.Write(padding[0:pad]); err != nil {
					return err
				}

			}
		}
	}
	return nil
}
