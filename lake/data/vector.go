package data

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/bufwriter"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/vng"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/vngio"
	"github.com/brimdata/super/zio/zngio"
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
	w, err := NewVectorWriter(ctx, engine, path, id)
	if err != nil {
		get.Close()
		return err
	}
	// Note here that writer.Close closes the Put but reader.Close does not
	// close the Get.
	reader := zngio.NewReader(super.NewContext(), get)
	err = zio.Copy(w, reader)
	if closeErr := w.Close(); err == nil {
		err = closeErr
	}
	if closeErr := reader.Close(); err == nil {
		err = closeErr
	}
	if closeErr := get.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		w.Abort()
	}
	return err
}

type VectorWriter struct {
	*vng.Writer
	delete func()
}

func (o *Object) NewVectorWriter(ctx context.Context, engine storage.Engine, path *storage.URI) (*VectorWriter, error) {
	return NewVectorWriter(ctx, engine, path, o.ID)
}

func NewVectorWriter(ctx context.Context, engine storage.Engine, path *storage.URI, id ksuid.KSUID) (*VectorWriter, error) {
	put, err := engine.Put(ctx, VectorURI(path, id))
	if err != nil {
		return nil, err
	}
	delete := func() {
		DeleteVector(context.Background(), engine, path, id)
	}
	return &VectorWriter{
		Writer: vngio.NewWriter(bufwriter.New(put)),
		delete: delete,
	}, nil
}

func (w *VectorWriter) Abort() {
	w.Close()
	w.delete()
}

func DeleteVector(ctx context.Context, engine storage.Engine, path *storage.URI, id ksuid.KSUID) error {
	if err := engine.Delete(ctx, VectorURI(path, id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
