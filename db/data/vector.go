package data

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/bufwriter"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sio/csupio"
	"github.com/brimdata/super/vector"
	"github.com/segmentio/ksuid"
)

// CreateVector writes the vectorized form of an existing Object in the CSUP format.
func CreateVector(ctx context.Context, engine storage.Engine, path *storage.URI, id ksuid.KSUID) error {
	get, err := engine.Get(ctx, SequenceURI(path, id))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Make a cleaner error.
			err = fmt.Errorf("object %s: %w", id, fs.ErrNotExist)
		}
		return err
	}
	w, err := NewVectorWriter(ctx, engine, path, id) //Pusher
	if err != nil {
		get.Close()
		return err
	}
	// Note here that writer.Close closes the Put but reader.Close does not
	// close the Get.
	sctx := super.NewContext()
	reader := bsupio.NewReader(sctx, get)
	puller := sbuf.NewDematerializer(sctx, sbuf.NewPuller(reader))
	for {
		var vec vector.Any
		vec, err = puller.Pull(false)
		if vec == nil || err != nil {
			break
		}
		err = w.Push(vec)
		if err != nil {
			break
		}
	}
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
	*csup.Serializer
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
		Serializer: csupio.NewSerializer(bufwriter.New(put)),
		delete:     delete,
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
