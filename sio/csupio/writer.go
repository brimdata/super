package csupio

import (
	"io"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/csup/rows"
)

// NewSerializer returns a new CSUP serializer that outputs to w.
func NewSerializer(w io.WriteCloser) *csup.Serializer {
	return csup.NewSerializer(w)
}

type Writer struct {
	*rows.Writer
}

func NewRowWriter(w io.WriteCloser) *Writer {
	return &Writer{rows.NewWriter(w)}
}
