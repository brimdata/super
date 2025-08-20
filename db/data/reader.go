package data

import (
	"context"
	"fmt"
	"io"

	"github.com/brimdata/super/db/seekindex"
	"github.com/brimdata/super/pkg/storage"
)

type Reader struct {
	io.Reader
	io.Closer
	TotalBytes int64
	ReadBytes  int64
}

// NewReader returns a Reader for this data object. If the object has a seek index
// and if the provided span skips part of the object, the seek index will be used to
// limit the reading window of the returned reader.
func (o *Object) NewReader(ctx context.Context, engine storage.Engine, path *storage.URI, ranges []seekindex.Range) (*Reader, error) {
	objectPath := o.SequenceURI(path)
	reader, err := engine.Get(ctx, objectPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", objectPath, err)
	}
	var r io.Reader
	var readBytes int64
	if len(ranges) == 0 {
		r = reader
		readBytes = o.Size
	} else {
		readers := make([]io.Reader, 0, len(ranges))
		for _, rg := range ranges {
			readers = append(readers, io.NewSectionReader(reader, rg.Offset, rg.Length))
			readBytes += rg.Length
		}
		r = io.MultiReader(readers...)
	}
	return &Reader{
		Reader:     r,
		Closer:     reader,
		TotalBytes: o.Size,
		ReadBytes:  readBytes,
	}, nil
}
