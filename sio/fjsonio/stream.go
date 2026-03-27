package fjsonio

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"

	"github.com/brimdata/super/vector"
)

type stream struct {
	r      io.Reader
	ch     chan result
	once   sync.Once
	ctx    context.Context
	cancel context.CancelFunc
}

func newStream(ctx context.Context, r io.Reader, n int) *stream {
	ctx, cancel := context.WithCancel(ctx)
	return &stream{
		r:      r,
		ch:     make(chan result, n),
		ctx:    ctx,
		cancel: cancel,
	}
}

type result struct {
	bytes *vector.BytesTable
	err   error
}

func (s *stream) next() (*vector.BytesTable, error) {
	s.once.Do(func() {
		s.ch = make(chan result, runtime.GOMAXPROCS(0))
		go s.run()
	})
	select {
	case r, ok := <-s.ch:
		if errors.Is(r.err, io.EOF) {
			r.err = nil
		}
		if !ok || r.err != nil {
			return nil, r.err
		}
		return r.bytes, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *stream) run() {
	r := newValReader(s.r)
	for {
		batch, err := readBatch(r)
		select {
		case s.ch <- result{&batch, err}:
		case <-s.ctx.Done():
			return
		}
		if err != nil {
			close(s.ch)
			break
		}
	}
}

func (s *stream) close() error {
	s.cancel()
	// drain channel
	for range s.ch {
	}
	if closer, ok := s.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func readBatch(r *valReader) (vector.BytesTable, error) {
	// XXX Should we pool these?
	t := vector.NewBytesTableEmpty(VecBatchSize)
	for range VecBatchSize {
		b, err := r.Next()
		if err != nil {
			return t, err
		}
		t.Append(b)
	}
	return t, nil
}
