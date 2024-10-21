package parquetio

import (
	"context"
	"errors"
	"io"

	"github.com/apache/arrow/go/v14/arrow/memory"
	"github.com/apache/arrow/go/v14/parquet"
	"github.com/apache/arrow/go/v14/parquet/file"
	"github.com/apache/arrow/go/v14/parquet/pqarrow"
	"github.com/brimdata/super"
	"github.com/brimdata/super/zio/arrowio"
)

func NewReader(zctx *super.Context, r io.Reader) (*arrowio.Reader, error) {
	ras, ok := r.(parquet.ReaderAtSeeker)
	if !ok {
		return nil, errors.New("reader cannot seek")
	}
	pr, err := file.NewParquetReader(ras)
	if err != nil {
		return nil, err
	}
	props := pqarrow.ArrowReadProperties{
		Parallel:  true,
		BatchSize: 256 * 1024,
	}
	fr, err := pqarrow.NewFileReader(pr, props, memory.DefaultAllocator)
	if err != nil {
		pr.Close()
		return nil, err
	}
	rr, err := fr.GetRecordReader(context.TODO(), nil, nil)
	if err != nil {
		pr.Close()
		return nil, err
	}
	ar, err := arrowio.NewReaderFromRecordReader(zctx, rr)
	if err != nil {
		pr.Close()
		return nil, err
	}
	return ar, nil
}
