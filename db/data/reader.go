package data

import (
	"context"
	"fmt"
	"io"

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
func (o *Object) NewReader(ctx context.Context, engine storage.Engine, path *storage.URI) (*Reader, error) {
	objectPath := o.SequenceURI(path)
	reader, err := engine.Get(ctx, objectPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", objectPath, err)
	}
	return &Reader{
		Reader:     reader,
		Closer:     reader,
		TotalBytes: o.Size,
		ReadBytes:  o.Size,
	}, nil
}
