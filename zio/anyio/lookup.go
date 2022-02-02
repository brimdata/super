package anyio

import (
	"fmt"
	"io"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/csvio"
	"github.com/brimdata/zed/zio/jsonio"
	"github.com/brimdata/zed/zio/parquetio"
	"github.com/brimdata/zed/zio/zeekio"
	"github.com/brimdata/zed/zio/zjsonio"
	"github.com/brimdata/zed/zio/zng21io"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zio/zsonio"
	"github.com/brimdata/zed/zio/zstio"
)

func lookupReader(r io.Reader, zctx *zed.Context, opts ReaderOpts) (zio.Reader, error) {
	switch opts.Format {
	case "csv":
		return csvio.NewReader(r, zctx), nil
	case "zeek":
		return zeekio.NewReader(r, zctx), nil
	case "json":
		return jsonio.NewReader(r, zctx), nil
	case "zjson":
		return zjsonio.NewReader(r, zctx), nil
	case "zng":
		return zngio.NewReaderWithOpts(r, zctx, opts.ZNG), nil
	case "zng21":
		return zng21io.NewReaderWithOpts(r, zctx, opts.ZNG), nil
	case "zson":
		return zsonio.NewReader(r, zctx), nil
	case "zst":
		return zstio.NewReader(r, zctx)
	case "parquet":
		return parquetio.NewReader(r, zctx)
	}
	return nil, fmt.Errorf("no such format: \"%s\"", opts.Format)
}
