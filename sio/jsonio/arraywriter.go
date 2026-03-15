package jsonio

import (
	"bytes"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/vector"
)

type ArrayWriter struct {
	buf   *bytes.Buffer
	w     *Writer
	wc    io.WriteCloser
	wrote bool
}

func NewArrayWriter(wc io.WriteCloser) *ArrayWriter {
	var buf bytes.Buffer
	return &ArrayWriter{
		buf: &buf,
		w:   NewWriter(sio.NopCloser(&buf), WriterOpts{}),
		wc:  wc,
	}
}

func (a *ArrayWriter) Close() error {
	s := "[]\n"
	if a.wrote {
		s = "]\n"
	}
	if _, err := io.WriteString(a.wc, s); err != nil {
		return err
	}
	return a.wc.Close()
}

func (a *ArrayWriter) Push(vec vector.Any) error {
	return sbuf.WriteVec(a, vec)
}

func (a *ArrayWriter) Write(val super.Value) error {
	a.buf.Reset()
	if a.wrote {
		a.buf.WriteByte(',')
	} else {
		a.buf.WriteByte('[')
		a.wrote = true
	}
	if err := a.w.Write(val); err != nil {
		return err
	}
	_, err := a.wc.Write(bytes.TrimSpace(a.buf.Bytes()))
	return err
}
