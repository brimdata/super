package csup

import (
	"errors"
	"io"
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/agg"
)

func FusedType(sctx *super.Context, r io.ReaderAt) (super.Type, error) {
	s, ok := r.(io.Seeker)
	if !ok {
		return nil, errors.New("reading file type requires seekable input")
	}
	size, err := s.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	if _, err := s.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	fuser := agg.NewFuser(super.NewContext(), false)
	for size > 0 {
		typ, n, err := readPart(sctx, r, size)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return nil, err
		}
		fuser.Fuse(typ)
		size -= n
	}
	return fuser.Type(), nil
}

func readPart(sctx *super.Context, s io.ReaderAt, size int64) (super.Type, int64, error) {
	t, err := ReadTrailer(io.NewSectionReader(s, size-TrailerSize, math.MaxInt64))
	if err != nil {
		return nil, 0, err
	}
	bytes := make([]byte, t.MetaSize)
	if _, err := io.ReadFull(io.NewSectionReader(s, size-TrailerSize-int64(t.MetaSize), int64(t.MetaSize)), bytes); err != nil {
		return nil, 0, err
	}
	typ, err := sctx.LookupByValue(bytes)
	return typ, int64(t.Size), err
}
