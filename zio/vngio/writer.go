package vngio

import (
	"io"

	"github.com/brimdata/zed/vng"
)

// NewWriter returns a writer to w.
func NewWriter(w io.WriteCloser) *vng.Writer {
	return vng.NewWriter(w)
}
