package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

var errStdioNotSupport = errors.New("method not supported with stdio source")

type StdioEngine struct {
	stdinMu       sync.Mutex
	stdinBytes    []byte
	stdinFileInfo os.FileInfo
}

func NewStdioEngine() *StdioEngine {
	return &StdioEngine{}
}

func (s *StdioEngine) Get(_ context.Context, u *URI) (Reader, error) {
	if u.Path != "stdin" && u.Path != "" {
		return nil, fmt.Errorf("cannot read from %q", u)
	}
	s.stdinMu.Lock()
	defer s.stdinMu.Unlock()
	if s.stdinFileInfo == nil {
		fi, err := os.Stdin.Stat()
		if err != nil {
			return nil, err
		}
		s.stdinFileInfo = fi
	}
	if fi := s.stdinFileInfo; fi.Mode().IsRegular() {
		// Regular files can seek.  Wrap with io.SectionReader for an
		// unshared Read offset.
		r := io.NewSectionReader(os.Stdin, 0, fi.Size())
		return &nopCloseReader{r, r, r}, nil
	}
	if s.stdinBytes == nil {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		s.stdinBytes = b
	}
	r := bytes.NewReader(s.stdinBytes)
	return &nopCloseReader{r, r, r}, nil
}

type nopCloseReader struct {
	io.Reader
	io.ReaderAt
	io.Seeker
}

func (*nopCloseReader) Close() error { return nil }

func (*StdioEngine) Put(ctx context.Context, u *URI) (io.WriteCloser, error) {
	switch u.Path {
	case "stdout", "":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		return nil, fmt.Errorf("cannot write to '%s'", u.Path)
	}
}

func (*StdioEngine) PutIfNotExists(context.Context, *URI, []byte) error {
	return errStdioNotSupport
}

func (*StdioEngine) Delete(ctx context.Context, u *URI) error {
	return errStdioNotSupport
}

func (*StdioEngine) DeleteByPrefix(ctx context.Context, u *URI) error {
	return errStdioNotSupport
}

func (*StdioEngine) Size(_ context.Context, u *URI) (int64, error) {
	return 0, errStdioNotSupport
}

func (*StdioEngine) Exists(_ context.Context, u *URI) (bool, error) {
	return true, nil
}

func (*StdioEngine) List(_ context.Context, _ *URI) ([]Info, error) {
	return nil, errStdioNotSupport
}
