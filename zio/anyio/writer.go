package anyio

import (
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/arrowio"
	"github.com/brimdata/super/zio/csvio"
	"github.com/brimdata/super/zio/jsonio"
	"github.com/brimdata/super/zio/lakeio"
	"github.com/brimdata/super/zio/lineio"
	"github.com/brimdata/super/zio/parquetio"
	"github.com/brimdata/super/zio/tableio"
	"github.com/brimdata/super/zio/textio"
	"github.com/brimdata/super/zio/vngio"
	"github.com/brimdata/super/zio/zeekio"
	"github.com/brimdata/super/zio/zjsonio"
	"github.com/brimdata/super/zio/zngio"
	"github.com/brimdata/super/zio/zsonio"
)

type WriterOpts struct {
	Format string
	Lake   lakeio.WriterOpts
	CSV    csvio.WriterOpts
	JSON   jsonio.WriterOpts
	ZNG    *zngio.WriterOpts // Nil means use defaults via zngio.NewWriter.
	ZSON   zsonio.WriterOpts
}

func NewWriter(w io.WriteCloser, opts WriterOpts) (zio.WriteCloser, error) {
	switch opts.Format {
	case "arrows":
		return arrowio.NewWriter(w), nil
	case "bsup":
		if opts.ZNG == nil {
			return zngio.NewWriter(w), nil
		}
		return zngio.NewWriterWithOpts(w, *opts.ZNG), nil
	case "csup":
		return vngio.NewWriter(w), nil
	case "csv":
		return csvio.NewWriter(w, opts.CSV), nil
	case "jsup", "":
		return zsonio.NewWriter(w, opts.ZSON), nil
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
