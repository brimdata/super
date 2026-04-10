package fjsonio

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"sync/atomic"

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
	batch *batch
	err   error
}

func (s *stream) next() (*batch, error) {
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

type batch struct {
	vector.BytesTable
	refs atomic.Int64
}

var batchPool sync.Pool

func newBatch() *batch {
	b, ok := batchPool.Get().(*batch)
	if !ok {
		b = &batch{
			BytesTable: vector.NewBytesTableEmpty(VecBatchSize),
		}
	}
	b.Reset()
	b.refs.Store(1)
	return b
}

func (b *batch) Done() {
	batchPool.Put(b)
}

func readBatch(r *valReader) (*batch, error) {
	t := newBatch()
	for range VecBatchSize {
		b, err := r.Next()
		if err != nil {
			return t, err
		}
		t.Append(b)
	}
	return t, nil
}
