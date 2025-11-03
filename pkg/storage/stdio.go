package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

var errStdioNotSupport = errors.New("method not supported with stdio source")

type StdioEngine struct{}

func NewStdioEngine() *StdioEngine {
	return &StdioEngine{}
}

func (*StdioEngine) Get(_ context.Context, u *URI) (Reader, error) {
	if u.Scheme != "stdio" || (u.Path != "stdin" && u.Path != "") {
		return nil, fmt.Errorf("cannot read from %q", u)
	}
	return os.Stdin, nil
}

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
