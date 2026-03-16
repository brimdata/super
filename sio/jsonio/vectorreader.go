package jsonio

import (
	"errors"
	"io"

	"github.com/bytedance/sonic/decoder"
)

type sonicReader struct {
	r      io.Reader
	buf    []byte
	cursor []byte
	EOF    bool
}

func newSonicReader(r io.Reader) *sonicReader {
	return &sonicReader{r: r, buf: make([]byte, 512*1024)}
}

func (r *sonicReader) Next() ([]byte, error) {
	if len(r.cursor) == 0 {
		if err := r.fill(); err != nil {
			return nil, err
		}
	}
	start, end := decoder.Skip(r.cursor)
	if start < 0 {
		// XXX When start < 0 the values indicate different error codes. Figure
		// these out.
		return nil, errors.New("hi")
	}
	b := r.cursor[start:end]
	r.cursor = r.cursor[end:]
	return b, nil
}

func (r *sonicReader) fill() error {
	if r.EOF {
		return io.EOF
	}
	// copy rest of cursor to buf
	copy(r.buf, r.cursor)
	n, err := r.r.Read(r.buf[len(r.cursor):])
	if err != nil {
		if errors.Is(err, io.EOF) {
			r.EOF = true
			err = nil
		}
		// XXX handle EOF properly.
		return err
	}
	r.cursor = r.buf
	r.cursor = r.cursor[:n]
	return nil
}
