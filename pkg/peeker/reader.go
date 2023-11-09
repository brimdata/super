package peeker

import (
	"errors"
	"io"
	"slices"
)

type Reader struct {
	io.Reader
	limit  int
	buffer []byte
	cursor []byte
	eof    bool
}

var (
	ErrBufferOverflow = errors.New("buffer too big")
	ErrTruncated      = errors.New("truncated input")
)

func NewReader(reader io.Reader, size, max int) *Reader {
	b := make([]byte, size)
	return &Reader{
		Reader: reader,
		limit:  max,
		buffer: b,
		cursor: b[:0],
	}
}

func (r *Reader) Reset() {
	r.cursor = r.buffer[:0]
	r.eof = false
}

func (r *Reader) Limit() int {
	return r.limit
}

func (r *Reader) fill(need int) error {
	if need > r.limit {
		return ErrBufferOverflow
	}
	r.buffer = slices.Grow(r.buffer[:0], need)
	r.buffer = r.buffer[:cap(r.buffer)]
	copy(r.buffer, r.cursor)
	clen := len(r.cursor)
	n, err := io.ReadAtLeast(r.Reader, r.buffer[clen:], need-clen)
	if err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			return err
		}
		r.eof = true
	}
	r.buffer = r.buffer[:clen+n]
	r.cursor = r.buffer
	return nil
}

func (r *Reader) Peek(n int) ([]byte, error) {
	if n < 0 {
		return nil, errors.New("peeker: negative length")
	}
	if len(r.cursor) == 0 && r.eof {
		return nil, io.EOF
	}
	if n > len(r.cursor) && !r.eof {
		if err := r.fill(n); err != nil {
			return nil, err
		}
		if len(r.cursor) == 0 && r.eof {
			return nil, io.EOF
		}
	}
	if n > len(r.cursor) {
		return r.cursor, ErrTruncated
	}
	return r.cursor[:n], nil
}

func (r *Reader) Read(n int) ([]byte, error) {
	b, err := r.Peek(n)
	if err != nil {
		return nil, err
	}
	r.cursor = r.cursor[n:]
	return b, nil
}

// ReadByte implements io.ByteReader.ReadByte.
func (r *Reader) ReadByte() (byte, error) {
	if len(r.cursor) > 0 {
		b := r.cursor[0]
		r.cursor = r.cursor[1:]
		return b, nil
	}
	buf, err := r.Read(1)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}
