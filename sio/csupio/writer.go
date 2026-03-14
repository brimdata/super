package csupio

import (
	"io"

	"github.com/brimdata/super/csup"
)

// NewSerializer returns a new CSUP serializer that outputs to w.
func NewSerializer(w io.WriteCloser) *csup.Serializer {
	return csup.NewSerializer(w)
}
