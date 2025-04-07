package csupio

import (
	"io"
	"math"
	"sync"

	"github.com/brimdata/super/csup"
)

type stream struct {
	mu  sync.Mutex
	r   io.ReaderAt
	off int64
}

func (s *stream) next() (*csup.Object, error) {

	s.mu.Lock()
	defer s.mu.Unlock()
	// We read the next object by creating a section reader that starts
	// at the end of the previous object without an upper bound.  The csup
	// package won't read off the end of the object so this is not a problem.
	o, err := csup.NewObject(io.NewSectionReader(s.r, s.off, math.MaxInt64))
	if err != nil {
		if err == io.EOF {
			err = nil
		}
		return nil, err
	}
	s.off += int64(o.Size())
	return o, nil
}
