package s3io

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteInvalidPath(t *testing.T) {
	_, err := NewWriter(t.Context(), "http://localhost/upload", nil)
	require.Equal(t, ErrInvalidS3Path, err)
}

func TestWriteSimple(t *testing.T) {
	results := bytes.NewBuffer(nil)
	expected := []byte("some test data")
	w, _ := NewWriter(t.Context(), "s3://localhost/upload", nil)
	w.uploader = mockUploader(func(_ context.Context, in *s3manager.UploadInput, _ ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
		_, err := io.Copy(results, in.Body)
		return &s3manager.UploadOutput{}, err
	})
	_, err := w.Write(expected)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.Equal(t, expected, results.Bytes())

}

func TestWriteImmediateError(t *testing.T) {
	expected := errors.New("expected error")
	w, _ := NewWriter(t.Context(), "s3://localhost/upload", nil)
	w.uploader = mockUploader(func(_ context.Context, in *s3manager.UploadInput, _ ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
		return &s3manager.UploadOutput{}, expected
	})
	_, err := w.Write([]byte("test data"))
	assert.Equal(t, expected, err)
	assert.Equal(t, expected, w.Close())
}

func TestWriteEventualError(t *testing.T) {
	data := []byte("test data")
	expected := errors.New("expected error")
	w, _ := NewWriter(t.Context(), "s3://localhost/upload", nil)
	w.uploader = mockUploader(func(_ context.Context, in *s3manager.UploadInput, _ ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
		buf := make([]byte, len(data))
		_, _ = in.Body.Read(buf)
		return &s3manager.UploadOutput{}, expected
	})
	_, err := w.Write(data)
	require.NoError(t, err)
	_, err = w.Write(data)
	assert.Equal(t, expected, err)
	assert.Equal(t, expected, w.Close())
}

type mockUploader func(context.Context, *s3manager.UploadInput, ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)

func (m mockUploader) UploadWithContext(ctx context.Context, in *s3manager.UploadInput, opts ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	return m(ctx, in, opts...)
}
