package csupio

import (
	"context"
	"io"
	"math"
	"runtime"
	"sync"

	"github.com/brimdata/super/csup"
)

type stream struct {
	r    io.ReaderAt
	ch   chan result
	once sync.Once
	ctx  context.Context
}

type result struct {
	off    int64
	header csup.DataHeader
	err    error
}

func (s *stream) next() (*csup.DataHeader, int64, error) {
	s.once.Do(func() {
		s.ch = make(chan result, runtime.GOMAXPROCS(0))
		go s.run()
	})
	select {
	case r, ok := <-s.ch:
		if !ok || r.err != nil {
			if r.err == io.EOF {
				return nil, -1, nil
			}
			return nil, -1, r.err
		}
		return &r.header, r.off, nil
	case <-s.ctx.Done():
		return nil, -1, s.ctx.Err()
	}
}

func (s *stream) run() {
	var off int64
	for {
		section, err := csup.ReadSection(io.NewSectionReader(s.r, off, math.MaxInt64))
		if err != nil || section.Type == csup.SectionObject {
			select {
			case s.ch <- result{off, section.Object, err}:
			case <-s.ctx.Done():
				return
			}
		}
		if err != nil {
			close(s.ch)
			break
		}
		off += int64(section.Size())
	}
}
