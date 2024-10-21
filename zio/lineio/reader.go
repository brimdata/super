package lineio

import (
	"bufio"
	"io"

	"github.com/brimdata/super"
)

type Reader struct {
	scanner *bufio.Scanner
	val     super.Value
}

func NewReader(r io.Reader) *Reader {
	s := bufio.NewScanner(r)
	s.Buffer(nil, 25*1024*1024)
	return &Reader{scanner: s}
}

func (r *Reader) Read() (*super.Value, error) {
	if !r.scanner.Scan() || r.scanner.Err() != nil {
		return nil, r.scanner.Err()
	}
	r.val = super.NewString(r.scanner.Text())
	return &r.val, nil
}
