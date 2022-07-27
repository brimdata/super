//go:generate mockgen -destination=./mock/mock_engine.go -package=mock github.com/brimdata/zed/pkg/storage Engine

package storage

import (
	"context"
	"errors"
	"io"
)

type Reader interface {
	io.Reader
	io.ReaderAt
	io.Closer
}

type Sizer interface {
	Size() (int64, error)
}

var ErrNotSupported = errors.New("method call on storage engine not supported")

type Engine interface {
	Get(context.Context, *URI) (Reader, error)
	Put(context.Context, *URI) (io.WriteCloser, error)
	PutIfNotExists(context.Context, *URI, []byte) error
	Delete(context.Context, *URI) error
	DeleteByPrefix(context.Context, *URI) error
	Exists(context.Context, *URI) (bool, error)
	Size(context.Context, *URI) (int64, error)
	List(context.Context, *URI) ([]Info, error)
}

type Info struct {
	Name string
	Size int64
}

func NewRemoteEngine() *Router {
	router := NewRouter()
	router.Enable(HTTPScheme)
	router.Enable(HTTPSScheme)
	router.Enable(S3Scheme)
	return router
}

func NewLocalEngine() *Router {
	router := NewRemoteEngine()
	router.Enable(FileScheme)
	router.Enable(StdioScheme)
	return router
}

func Put(ctx context.Context, engine Engine, u *URI, r io.Reader) error {
	w, err := engine.Put(ctx, u)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	if closeErr := w.Close(); err == nil {
		err = closeErr
	}
	return err
}

func Get(ctx context.Context, engine Engine, u *URI) ([]byte, error) {
	r, err := engine.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(r)
	if closeErr := r.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func Size(r Reader) (int64, error) {
	if sizer, ok := r.(Sizer); ok {
		return sizer.Size()
	}
	return 0, ErrNotSupported
}

// NewSeeker provides a seeker implementation on top of Reader.
// Using a seeker is not optimal as cloud-oriented apps should use
// application-level framing to do readahead and so forth based that
// leverages knowledge of the data of the underlying storage objects.
// This seeker interface is provided for backward compat with libraries
// like parquet-go that are based on an io.ReadSeeker.
func NewSeeker(r Reader) (*Seeker, error) {
	size, err := Size(r)
	if err != nil {
		return nil, err
	}
	return &Seeker{
		ReadSeeker: io.NewSectionReader(r, 0, size),
		Reader:     r,
	}, nil
}

type Seeker struct {
	io.ReadSeeker
	Reader
}

// Read resolves the ambiguous selector s.Read to s.ReadSeeker.Read.
func (s *Seeker) Read(b []byte) (int, error) {
	return s.ReadSeeker.Read(b)
}

func IsSeekable(r Reader) bool {
	// All of the Reader implementations are seekable except this one,
	// which is used by stdin.
	_, ok := r.(*nopReadAtCloser)
	return !ok
}
