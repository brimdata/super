package csupio

import (
	"io"

	"github.com/brimdata/super/csup"
)

// NewWriter returns a writer to w.
func NewWriter(w io.WriteCloser) *csup.Writer {
	return csup.NewWriter(w)
}
