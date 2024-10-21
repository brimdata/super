package anyio

import (
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/optimizer/demand"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/arrowio"
	"github.com/brimdata/super/zio/csvio"
	"github.com/brimdata/super/zio/jsonio"
	"github.com/brimdata/super/zio/lineio"
	"github.com/brimdata/super/zio/parquetio"
	"github.com/brimdata/super/zio/vngio"
	"github.com/brimdata/super/zio/zeekio"
	"github.com/brimdata/super/zio/zjsonio"
	"github.com/brimdata/super/zio/zngio"
	"github.com/brimdata/super/zio/zsonio"
)

func lookupReader(zctx *super.Context, r io.Reader, demandOut demand.Demand, opts ReaderOpts) (zio.ReadCloser, error) {
	switch opts.Format {
	case "arrows":
		return arrowio.NewReader(zctx, r)
	case "csv":
		return zio.NopReadCloser(csvio.NewReader(zctx, r, opts.CSV)), nil
	case "line":
		return zio.NopReadCloser(lineio.NewReader(r)), nil
	case "json":
		return zio.NopReadCloser(jsonio.NewReader(zctx, r)), nil
	case "parquet":
		zr, err := parquetio.NewReader(zctx, r)
		if err != nil {
			return nil, err
		}
		return zio.NopReadCloser(zr), nil
	case "tsv":
		opts.CSV.Delim = '\t'
		return zio.NopReadCloser(csvio.NewReader(zctx, r, opts.CSV)), nil
	case "vng":
		zr, err := vngio.NewReader(zctx, r, demandOut)
		if err != nil {
			return nil, err
		}
		return zio.NopReadCloser(zr), nil
	case "zeek":
		return zio.NopReadCloser(zeekio.NewReader(zctx, r)), nil
	case "zjson":
		return zio.NopReadCloser(zjsonio.NewReader(zctx, r)), nil
	case "zng":
		return zngio.NewReaderWithOpts(zctx, r, opts.ZNG), nil
	case "zson":
		return zio.NopReadCloser(zsonio.NewReader(zctx, r)), nil
	}
	return nil, fmt.Errorf("no such format: \"%s\"", opts.Format)
}
