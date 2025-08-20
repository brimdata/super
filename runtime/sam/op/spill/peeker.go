package spill

import (
	"context"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sio"
)

type peeker struct {
	*File
	nextRecord *super.Value
	ordinal    int
}

func newPeeker(ctx context.Context, sctx *super.Context, filename string, ordinal int, zr sio.Reader) (*peeker, error) {
	f, err := NewFileWithPath(filename)
	if err != nil {
		return nil, err
	}
	if err := sio.CopyWithContext(ctx, f, zr); err != nil {
		f.CloseAndRemove()
		return nil, err
	}
	if err := f.Rewind(sctx); err != nil {
		f.CloseAndRemove()
		return nil, err
	}
	first, err := f.Read()
	if err != nil {
		f.CloseAndRemove()
		return nil, err
	}
	return &peeker{f, first, ordinal}, nil
}

// read is like Read but returns eof at the last record so a MergeSort can
// do its heap management a bit more easily.
func (p *peeker) read() (*super.Value, bool, error) {
	rec := p.nextRecord
	if rec != nil {
		rec = rec.Copy().Ptr()
	}
	var err error
	p.nextRecord, err = p.Read()
	eof := p.nextRecord == nil && err == nil
	return rec, eof, err
}
