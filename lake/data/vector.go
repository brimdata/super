package data

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/bufwriter"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/vngio"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/segmentio/ksuid"
)

// CreateVector writes the vectorized form of an existing Object in the VNG format.
func CreateVector(ctx context.Context, engine storage.Engine, path *storage.URI, id ksuid.KSUID) error {
	get, err := engine.Get(ctx, SequenceURI(path, id))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Make a cleaner error.
			err = fmt.Errorf("object %s: %w", id, fs.ErrNotExist)
		}
		return err
	}
	put, err := engine.Put(ctx, VectorURI(path, id))
	if err != nil {
		get.Close()
		return err
	}
	writer, err := vngio.NewWriter(bufwriter.New(put), vngio.WriterOpts{
		ColumnThresh: vngio.DefaultColumnThresh,
		SkewThresh:   vngio.DefaultSkewThresh,
	})
	if err != nil {
		get.Close()
		put.Close()
		DeleteVector(ctx, engine, path, id)
		return err
	}
	// Note here that writer.Close closes the Put but reader.Close does not
	// close the Get.
	reader := zngio.NewReader(zed.NewContext(), get)
	err = zio.Copy(writer, reader)
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if closeErr := reader.Close(); err == nil {
		err = closeErr
	}
	if closeErr := get.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		DeleteVector(ctx, engine, path, id)
	}
	return err
}

func DeleteVector(ctx context.Context, engine storage.Engine, path *storage.URI, id ksuid.KSUID) error {
	if err := engine.Delete(ctx, VectorURI(path, id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
