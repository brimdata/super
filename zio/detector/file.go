package detector

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/brimsec/zq/pkg/fs"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio/ndjsonio"
	"github.com/brimsec/zq/zio/parquetio"
	"github.com/brimsec/zq/zng/resolver"

	"github.com/xitongsys/parquet-go-source/local"
	parquets3 "github.com/xitongsys/parquet-go-source/s3"
	"github.com/xitongsys/parquet-go/source"
)

type OpenConfig struct {
	Format         string
	JSONTypeConfig *ndjsonio.TypeConfig
	JSONPathRegex  string
	AwsCfg         *aws.Config
}

func IsS3Path(path string) bool {
	u, err := url.Parse(path)
	if err != nil {
		return false
	}
	return u.Scheme == "s3"
}

const StdinPath = "/dev/stdin"

// OpenFile creates and returns zbuf.File for the indicated "path",
// which can be a local file path, a local directory path, or an S3
// URL. If the path is neither of these or can't otherwise be opened,
// an error is returned.
func OpenFile(zctx *resolver.Context, path string, cfg OpenConfig) (*zbuf.File, error) {
	// Parquet is special and needs its own reader for s3 sources- therefore this must go before
	// the IsS3Path check.
	if cfg.Format == "parquet" {
		return OpenParquet(zctx, path, cfg)
	}

	if IsS3Path(path) {
		return OpenS3File(zctx, path, cfg)
	}

	var f *os.File
	if path == StdinPath {
		f = os.Stdin
	} else {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			return nil, errors.New("is a directory")
		}
		f, err = fs.Open(path)
		if err != nil {
			return nil, err
		}
	}

	return OpenFromNamedReadCloser(zctx, f, path, cfg)
}

type pipeWriterAt struct {
	*io.PipeWriter
}

func (pw *pipeWriterAt) WriteAt(p []byte, _ int64) (n int, err error) {
	return pw.Write(p)
}

// OpenS3File opens a file pointed to by an S3-style URL like s3://bucket/name.
//
// The AWS SDK requires the region and credentials (access key ID and
// secret) to make a request to S3. They can be passed as the usual
// AWS environment variables, or be read from the usual aws config
// files in ~/.aws.
//
// Note that access to public objects without credentials is possible
// only if awscfg.AwsCfg.Credentials is set to
// credentials.AnonymousCredentials. However, use of anonymous
// credentials is currently not exposed as a zq command-line option,
// and any attempt to read from S3 without credentials fails.
// (Another way to access such public objects would be through plain
// https access, once we add that support).
func OpenS3File(zctx *resolver.Context, s3path string, cfg OpenConfig) (*zbuf.File, error) {
	u, err := url.Parse(s3path)
	if err != nil {
		return nil, err
	}
	sess := session.Must(session.NewSession(cfg.AwsCfg))
	s3Downloader := s3manager.NewDownloader(sess)
	getObj := &s3.GetObjectInput{
		Bucket: aws.String(u.Host),
		Key:    aws.String(u.Path),
	}
	pr, pw := io.Pipe()
	go func() {
		_, err := s3Downloader.Download(&pipeWriterAt{pw}, getObj, func(d *s3manager.Downloader) {
			d.Concurrency = 1
		})
		pw.CloseWithError(err)
	}()
	return OpenFromNamedReadCloser(zctx, pr, s3path, cfg)
}

func OpenParquet(zctx *resolver.Context, path string, cfg OpenConfig) (*zbuf.File, error) {
	var pf source.ParquetFile
	var err error
	if IsS3Path(path) {
		var u *url.URL
		u, err = url.Parse(path)
		if err != nil {
			return nil, err
		}
		pf, err = parquets3.NewS3FileReader(context.Background(), u.Host, u.Path, cfg.AwsCfg)
	} else {
		pf, err = local.NewLocalFileReader(path)
	}
	if err != nil {
		return nil, err
	}

	r, err := parquetio.NewReader(pf, zctx, parquetio.ReaderOpts{})
	if err != nil {
		return nil, err
	}
	return zbuf.NewFile(r, pf, path), nil
}

func OpenFromNamedReadCloser(zctx *resolver.Context, rc io.ReadCloser, path string, cfg OpenConfig) (*zbuf.File, error) {
	var err error
	r := GzipReader(rc)
	var zr zbuf.Reader
	if cfg.Format == "" || cfg.Format == "auto" {
		zr, err = NewReaderWithConfig(r, zctx, path, cfg)
	} else {
		zr, err = lookupReader(r, zctx, path, cfg)
	}
	if err != nil {
		return nil, err
	}

	return zbuf.NewFile(zr, rc, path), nil
}

func OpenFiles(zctx *resolver.Context, dir zbuf.RecordCmpFn, paths ...string) (*zbuf.Combiner, error) {
	var readers []zbuf.Reader
	for _, path := range paths {
		reader, err := OpenFile(zctx, path, OpenConfig{})
		if err != nil {
			return nil, err
		}
		readers = append(readers, reader)
	}
	return zbuf.NewCombiner(readers, dir), nil
}
