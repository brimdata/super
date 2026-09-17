package storage

import (
	"context"
	"io"
	"io/fs"
)

type Open func() (Reader, error)

type InternalEngine struct {
	openers map[string]Open
}

func NewInternalEngine() *InternalEngine {
	return &InternalEngine{map[string]Open{}}
}

func (i *InternalEngine) AddFile(uri string, o Open) {
	i.openers[uri] = o
}

func (i *InternalEngine) Get(_ context.Context, u *URI) (Reader, error) {
	o, ok := i.openers[u.String()]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return o()
}

func (*InternalEngine) Put(context.Context, *URI) (io.WriteCloser, error) {
	return nil, ErrNotSupported
}

func (*InternalEngine) PutIfNotExists(context.Context, *URI, []byte) error {
	return ErrNotSupported

}
func (*InternalEngine) Delete(context.Context, *URI) error {
	return ErrNotSupported
}

func (*InternalEngine) DeleteByPrefix(context.Context, *URI) error {
	return ErrNotSupported
}

func (i *InternalEngine) Exists(_ context.Context, u *URI) (bool, error) {
	_, ok := i.openers[u.String()]
	return ok, nil
}

func (i *InternalEngine) Size(_ context.Context, u *URI) (int64, error) {
	_, ok := i.openers[u.String()]
	if !ok {
		return 0, fs.ErrNotExist
	}
	return 0, nil
}

func (*InternalEngine) List(context.Context, *URI) ([]Info, error) {
	return nil, ErrNotSupported
}
