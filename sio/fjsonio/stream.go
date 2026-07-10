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
	bytes        *vector.BytesTable
	startLineNum int
	err          error
}

func (s *stream) next() (*vector.BytesTable, int, error) {
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
			return nil, -1, r.err
		}
		return r.bytes, r.startLineNum, nil
	case <-s.ctx.Done():
		return nil, -1, s.ctx.Err()
	case <-s.done:
		return nil, -1, nil
	}
}

func (s *stream) run() {
	r := newValReader(s.r)
	for {
		batch, startLineNum, err := readBatch(r)
		select {
		case s.ch <- result{batch, startLineNum, err}:
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
	return nil
}

var bytesTablePool sync.Pool

func newBytesTable() *vector.BytesTable {
	b, ok := bytesTablePool.Get().(*vector.BytesTable)
	if !ok {
		b = new(vector.NewBytesTableEmpty(VecBatchSize))
	}
	b.Reset()
	return b
}

func readBatch(r *valReader) (*vector.BytesTable, int, error) {
	t := newBytesTable()
	start := r.lineNumber()
	for range VecBatchSize {
		b, err := r.Next()
		if err != nil {
			return t, start, err
		}
		t.Append(b)
	}
	return t, start, nil
}
