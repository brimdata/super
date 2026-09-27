package bsupio

import (
	"io"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/csup/rows"
)

// NewSerializer returns a new CSUP serializer that outputs to w.
func NewSerializer(w io.WriteCloser) *csup.Serializer {
	return csup.NewSerializer(w)
}

// XXX RowWriter provides a wrapper to the old BSUP format encapsulated by
// the new framing design.
type RowWriter struct {
	*rows.Writer
}

func NewRowWriter(w io.WriteCloser) *RowWriter {
	return &RowWriter{rows.NewWriter(w)}
}
