package s3io

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

type Reader struct {
	client *s3.S3
	ctx    context.Context
	bucket string
	key    string
	size   int64

	offset int64
	body   io.ReadCloser
}

func NewReader(ctx context.Context, path string, cfg *aws.Config) (*Reader, error) {
	info, err := Stat(ctx, path, cfg)
	if err != nil {
		return nil, err
	}
	bucket, key, err := parsePath(path)
	if err != nil {
		return nil, err
	}
	return &Reader{
		client: newClient(cfg),
		ctx:    ctx,
		bucket: bucket,
		key:    key,
		size:   info.Size,
	}, nil
}

func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += r.offset
	case io.SeekEnd:
		offset += r.size
	default:
		return 0, errors.New("s3io.Reader.Seek: invalid whence")
	}
	if offset < 0 {
		return 0, errors.New("s3io.Reader.Seek: negative position")
	}
	if offset == r.offset {
		return offset, nil
	}
	r.offset = offset
	if r.body != nil {
		r.body.Close()
		r.body = nil
	}
	return r.offset, nil
}

func (r *Reader) Read(p []byte) (int, error) {
	if r.offset >= r.size {
		return 0, io.EOF
	}
	if r.body == nil {
		body, err := r.makeRequest(r.offset, r.size-r.offset)
		if err != nil {
			return 0, err
		}
		r.body = body
	}
	n, err := r.body.Read(p)
	if err == io.EOF {
		err = nil
	}
	if err == nil {
		r.offset += int64(n)
	}
	return n, err
}

func (r *Reader) ReadAt(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if off >= r.size {
		return 0, io.EOF
	}
	count := int64(len(p))
	if off+count >= r.size {
		count = r.size - off
	}
	b, err := r.makeRequest(off, count)
	if err != nil {
		return 0, err
	}
	defer b.Close()
	return io.ReadAtLeast(b, p, int(count))
}

func (r *Reader) Close() error {
	var err error
	if r.body != nil {
		err = r.body.Close()
		r.body = nil
	}
	return err
}

func (r *Reader) makeRequest(off int64, count int64) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(r.key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", off, off+count-1)),
	}
	res, err := r.client.GetObjectWithContext(r.ctx, input)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}
