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
	"github.com/brimdata/super/zio/lineio"
	"github.com/brimdata/super/zio/parquetio"
	"github.com/brimdata/super/zio/supio"
	"github.com/brimdata/super/zio/zeekio"
	"github.com/brimdata/super/zio/zjsonio"
)

func lookupReader(sctx *super.Context, r io.Reader, opts ReaderOpts) (zio.ReadCloser, error) {
	switch opts.Format {
	case "arrows":
		return arrowio.NewReader(sctx, r)
	case "bsup":
		return bsupio.NewReaderWithOpts(sctx, r, opts.BSUP), nil
	case "csup":
		zr, err := csupio.NewReader(sctx, r, opts.Fields)
		if err != nil {
			return nil, err
		}
		return zio.NopReadCloser(zr), nil
	case "csv":
		return zio.NopReadCloser(csvio.NewReader(sctx, r, opts.CSV)), nil
	case "line":
		return zio.NopReadCloser(lineio.NewReader(r)), nil
	case "json":
		return zio.NopReadCloser(jsonio.NewReader(sctx, r)), nil
	case "parquet":
		zr, err := parquetio.NewReader(sctx, r, opts.Fields)
		if err != nil {
			return nil, err
		}
		return zio.NopReadCloser(zr), nil
	case "sup":
		return zio.NopReadCloser(supio.NewReader(sctx, r)), nil
	case "tsv":
		opts.CSV.Delim = '\t'
		return zio.NopReadCloser(csvio.NewReader(sctx, r, opts.CSV)), nil
	case "zeek":
		return zio.NopReadCloser(zeekio.NewReader(sctx, r)), nil
	case "zjson":
		return zio.NopReadCloser(zjsonio.NewReader(sctx, r)), nil
	}
	return nil, fmt.Errorf("no such format: \"%s\"", opts.Format)
}
