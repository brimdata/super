package zsonio

import (
	"io"
	"regexp"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zson"
)

type Writer struct {
	writer    io.WriteCloser
	formatter *zson.Formatter
}

type WriterOpts struct {
	ColorDisabled bool
	Pretty        int
	Persist       *regexp.Regexp
}

func NewWriter(w io.WriteCloser, opts WriterOpts) *Writer {
	return &Writer{
		formatter: zson.NewFormatter(opts.Pretty, opts.ColorDisabled, opts.Persist),
		writer:    w,
	}
}

func (w *Writer) Close() error {
	return w.writer.Close()
}

func (w *Writer) Write(rec super.Value) error {
	if _, err := io.WriteString(w.writer, w.formatter.FormatRecord(rec)); err != nil {
		return err
	}
	_, err := w.writer.Write([]byte("\n"))
	return err
}
