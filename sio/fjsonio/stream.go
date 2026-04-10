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
	r    io.Reader
	ch   chan result
	done chan struct{}
	once sync.Once
	ctx  context.Context
}

func newStream(ctx context.Context, r io.Reader, n int) *stream {
	return &stream{
		r:    r,
		ch:   make(chan result, n),
		ctx:  ctx,
		done: make(chan struct{}),
	}
}

type result struct {
	batch *vector.BytesTable
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
		return r.batch, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case <-s.done:
		return nil, nil
	}
}

func (s *stream) run() {
	r := newValReader(s.r)
	for {
		batch, err := readBatch(r)
		select {
		case s.ch <- result{batch, err}:
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
	close(s.done)
	// drain channel
	for range s.ch {
	}
	if closer, ok := s.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

var batchPool sync.Pool

func newBytesTable() *vector.BytesTable {
	b, ok := batchPool.Get().(*vector.BytesTable)
	if !ok {
		b = new(vector.NewBytesTableEmpty(VecBatchSize))
	}
	b.Reset()
	return b
}

func readBatch(r *valReader) (*vector.BytesTable, error) {
	t := newBytesTable()
	for range VecBatchSize {
		b, err := r.Next()
		if err != nil {
			return t, err
		}
		t.Append(b)
	}
	return t, nil
}
