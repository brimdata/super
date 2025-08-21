package journal

import (
	"context"
	"io"
)

// XXX we should add some concurrency here so the reader can do readahead
// instead of a round-trip for each small journal file.

type Reader struct {
	ctx    context.Context
	q      *Queue
	cursor ID
	stop   ID
	buffer []byte
}

func newReader(ctx context.Context, q *Queue, head, tail ID) *Reader {
	return &Reader{
		ctx:    ctx,
		q:      q,
		cursor: tail - 1,
		stop:   head,
	}
}

func (r *Reader) Read(b []byte) (int, error) {
	if len(r.buffer) == 0 {
		if err := r.fill(); err != nil {
			return 0, err
		}
	}
	n := copy(b, r.buffer)
	r.buffer = r.buffer[n:]
	return n, nil
}

func (r *Reader) fill() error {
	if r.cursor >= r.stop {
		return io.EOF
	}
	r.cursor++
	var err error
	r.buffer, err = r.q.Load(r.ctx, r.cursor)
	return err
}
