package parquetio

import (
	"context"
	"errors"
	"io"

	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/apache/arrow-go/v18/parquet/schema"
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/zio/arrowio"
)

func NewReader(zctx *super.Context, r io.Reader, fields []field.Path) (*arrowio.Reader, error) {
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
	cols := columnIndexes(pr.MetaData().Schema, fields)
	rr, err := fr.GetRecordReader(context.TODO(), cols, nil)
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

func columnIndexes(schema *schema.Schema, fields []field.Path) []int {
	var indexes []int
	for _, f := range fields {
		if i := schema.ColumnIndexByName(f.String()); i >= 0 {
			indexes = append(indexes, i)
		}
	}
	return indexes
}
