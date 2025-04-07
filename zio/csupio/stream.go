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
	// NewObject puts the right end to the passed in SectionReader and since
	// the end is unkown just pass MaxInt64.
	s.mu.Lock()
	defer s.mu.Unlock()
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
