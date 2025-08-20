package parquetio

import (
	"fmt"
	"io"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/brimdata/super"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/arrowio"
)

type Writer struct {
	*arrowio.Writer
}

func NewWriter(wc io.WriteCloser) *Writer {
	w := arrowio.NewWriter(wc)
	w.NewWriterFunc = func(w io.Writer, s *arrow.Schema) (arrowio.WriteCloser, error) {
		fw, err := pqarrow.NewFileWriter(s, sio.NopCloser(w), nil, pqarrow.DefaultWriterProps())
		if err != nil {
			return nil, fmt.Errorf("%w: %s", arrowio.ErrUnsupportedType, err)
		}
		return fw, nil
	}
	return &Writer{w}
}

func (w *Writer) Write(val super.Value) error {
	if err := w.Writer.Write(val); err != nil {
		return parquetioError{err}
	}
	return nil
}

type parquetioError struct {
	err error
}

func (p parquetioError) Error() string {
	return "parquetio: " + strings.TrimPrefix(p.err.Error(), "arrowio: ")
}

func (p parquetioError) Unwrap() error { return p.err }
