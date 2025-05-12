package anyio

import (
	"errors"
	"io"
	"slices"
)

var ErrBufferOverflow = errors.New("buffer exceeded max size trying to infer input format")

const MaxBufferSize = 10 * 1024 * 1024
const InitBufferSize = 8 * 1024

type Recorder struct {
	io.Reader
	eof    bool
	buffer []byte
}

func NewRecorder(r io.Reader) *Recorder {
	return &Recorder{
		Reader: r,
		buffer: make([]byte, 0, InitBufferSize),
	}
}

func (r *Recorder) ReadAt(off int, b []byte) (int, error) {
	for {
		if off < len(r.buffer) {
			n := copy(b, r.buffer[off:])
			return n, nil
		}
		if r.eof {
			return 0, io.EOF
		}
		if err := r.fill(); err != nil {
			return 0, err

		}
	}
}

func (r *Recorder) fill() error {
	for {
		off := len(r.buffer)
		n := cap(r.buffer)
		if off < n {
			cc, err := r.Reader.Read(r.buffer[off:n])
			if err == io.EOF {
				r.eof = true
				err = nil
			}
			r.buffer = r.buffer[:off+cc]
			return err
		}
		newsize := 2 * n
		for newsize < off+InitBufferSize {
			newsize *= 2
		}
		if newsize >= MaxBufferSize {
			return ErrBufferOverflow
		}
		r.buffer = slices.Grow(r.buffer, newsize-off)
	}
}

func (r *Recorder) Read(b []byte) (int, error) {
	if r.buffer == nil {
		return r.Reader.Read(b)
	}
	n := copy(b, r.buffer)
	r.buffer = r.buffer[n:]
	if len(r.buffer) == 0 {
		// no longer needed, return to GC
		r.buffer = nil
	}
	return n, nil
}
