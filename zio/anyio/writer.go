package anyio

import (
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/arrowio"
	"github.com/brimdata/super/zio/bsupio"
	"github.com/brimdata/super/zio/csupio"
	"github.com/brimdata/super/zio/csvio"
	"github.com/brimdata/super/zio/jsonio"
	"github.com/brimdata/super/zio/lakeio"
	"github.com/brimdata/super/zio/lineio"
	"github.com/brimdata/super/zio/parquetio"
	"github.com/brimdata/super/zio/supio"
	"github.com/brimdata/super/zio/tableio"
	"github.com/brimdata/super/zio/textio"
	"github.com/brimdata/super/zio/zeekio"
	"github.com/brimdata/super/zio/zjsonio"
)

type WriterOpts struct {
	Format string
	BSUP   *bsupio.WriterOpts // Nil means use defaults via bsupio.NewWriter.
	CSV    csvio.WriterOpts
	JSON   jsonio.WriterOpts
	Lake   lakeio.WriterOpts
	SUP    supio.WriterOpts
}

func NewWriter(w io.WriteCloser, opts WriterOpts) (zio.WriteCloser, error) {
	switch opts.Format {
	case "arrows":
		return arrowio.NewWriter(w), nil
	case "bsup":
		if opts.BSUP == nil {
			return bsupio.NewWriter(w), nil
		}
		return bsupio.NewWriterWithOpts(w, *opts.BSUP), nil
	case "csup":
		return csupio.NewWriter(w), nil
	case "csv":
		return csvio.NewWriter(w, opts.CSV), nil
	case "json":
		return jsonio.NewWriter(w, opts.JSON), nil
	case "lake":
		return lakeio.NewWriter(w, opts.Lake), nil
	case "line":
		return lineio.NewWriter(w), nil
	case "null":
		return &nullWriter{}, nil
	case "parquet":
		return parquetio.NewWriter(w), nil
	case "sup", "":
		return supio.NewWriter(w, opts.SUP), nil
	case "table":
		return tableio.NewWriter(w), nil
	case "text":
		return textio.NewWriter(w), nil
	case "tsv":
		opts.CSV.Delim = '\t'
		return csvio.NewWriter(w, opts.CSV), nil
	case "zeek":
		return zeekio.NewWriter(w), nil
	case "zjson":
		return zjsonio.NewWriter(w), nil
	default:
		return nil, fmt.Errorf("unknown format: %s", opts.Format)
	}
}

type nullWriter struct{}

func (*nullWriter) Write(super.Value) error {
	return nil
}

func (*nullWriter) Close() error {
	return nil
}
