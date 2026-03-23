package fjsonio

import (
	"errors"
	"io"

	"github.com/bytedance/sonic/decoder"
)

type valReader struct {
	r      io.Reader
	buf    []byte
	cursor []byte
	EOF    bool
}

func newValReader(r io.Reader) *valReader {
	return &valReader{r: r, buf: make([]byte, 512*1024)}
}

func (r *valReader) Next() ([]byte, error) {
	start, end := decoder.Skip(r.cursor)
	if start < 0 {
		// XXX There's an issue here if we encounter a value that is larger than
		// the default buffer size. We should probably include functionality to
		// increase the buffer size to an arbitrary amount and return a detailed
		// error if a value is larger than MaxBufSize.
		if err := r.fill(); err != nil {
			return nil, err
		}
		start, end = decoder.Skip(r.cursor)
		if start < 0 {
			return nil, errors.New("invalid input")
		}
	}
	b := r.cursor[start:end]
	r.cursor = r.cursor[end:]
	return b, nil
}

func (r *valReader) fill() error {
	if r.EOF {
		return io.EOF
	}
	// copy rest of cursor to buf
	cc := copy(r.buf, r.cursor)
	n, err := r.r.Read(r.buf[cc:])
	if errors.Is(err, io.EOF) {
		r.EOF = true
		if n == 0 {
			return err
		}
		err = nil
	}
	if err != nil {
		return err
	}
	r.cursor = r.buf
	r.cursor = r.cursor[:cc+n]
	return nil
}
