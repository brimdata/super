package storage

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/superdb/super/pkg/s3io"
)

type S3Engine struct {
	client s3iface.S3API
}

var _ Engine = (*S3Engine)(nil)
var _ Sizer = (*s3io.Reader)(nil)

func NewS3() *S3Engine {
	return &S3Engine{
		client: s3io.NewClient(nil),
	}
}

func (s *S3Engine) Get(ctx context.Context, u *URI) (Reader, error) {
	r, err := s3io.NewReader(ctx, u.String(), s.client)
	return r, s3Err(err)
}

func (s *S3Engine) Put(ctx context.Context, u *URI) (io.WriteCloser, error) {
	w, err := s3io.NewWriter(ctx, u.String(), s.client)
	return w, s3Err(err)
}

func (s *S3Engine) PutIfNotExists(context.Context, *URI, []byte) error {
	return ErrNotSupported
}

func (s *S3Engine) Delete(ctx context.Context, u *URI) error {
	return s3Err(s3io.Remove(ctx, u.String(), s.client))
}

func (s *S3Engine) DeleteByPrefix(ctx context.Context, u *URI) error {
	return s3Err(s3io.RemoveAll(ctx, u.String(), s.client))
}

func (s *S3Engine) Size(ctx context.Context, u *URI) (int64, error) {
	info, err := s3io.Stat(ctx, u.String(), s.client)
	return info.Size, s3Err(err)
}

func (s *S3Engine) Exists(ctx context.Context, u *URI) (bool, error) {
	ok, err := s3io.Exists(ctx, u.String(), s.client)
	return ok, s3Err(err)
}

func (s *S3Engine) List(ctx context.Context, uri *URI) ([]Info, error) {
	entries, err := s3io.List(ctx, uri.String(), s.client)
	if err != nil {
		return nil, err
	}
	infos := make([]Info, 0, len(entries))
	for _, e := range entries {
		infos = append(infos, Info{
			Name: e.Name,
			Size: e.Size,
		})
	}
	return infos, nil
}

func s3Err(err error) error {
	var reqerr awserr.RequestFailure
	if errors.As(err, &reqerr) && reqerr.StatusCode() == http.StatusNotFound {
		return fs.ErrNotExist
	}
	return err
}
