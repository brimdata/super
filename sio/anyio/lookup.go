package anyio

import (
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/arrowio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sio/csupio"
	"github.com/brimdata/super/sio/csvio"
	"github.com/brimdata/super/sio/jsonio"
	"github.com/brimdata/super/sio/jsupio"
	"github.com/brimdata/super/sio/lineio"
	"github.com/brimdata/super/sio/parquetio"
	"github.com/brimdata/super/sio/supio"
	"github.com/brimdata/super/sio/zeekio"
)

func lookupReader(sctx *super.Context, r io.Reader, opts ReaderOpts) (sio.ReadCloser, error) {
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
		return sio.NopReadCloser(zr), nil
	case "csv":
		return sio.NopReadCloser(csvio.NewReader(sctx, r, opts.CSV)), nil
	case "line":
		return sio.NopReadCloser(lineio.NewReader(r)), nil
	case "json":
		return sio.NopReadCloser(jsonio.NewReader(sctx, r)), nil
	case "parquet":
		zr, err := parquetio.NewReader(sctx, r, opts.Fields)
		if err != nil {
			return nil, err
		}
		return sio.NopReadCloser(zr), nil
	case "sup":
		return sio.NopReadCloser(supio.NewReader(sctx, r)), nil
	case "tsv":
		opts.CSV.Delim = '\t'
		return sio.NopReadCloser(csvio.NewReader(sctx, r, opts.CSV)), nil
	case "zeek":
		return sio.NopReadCloser(zeekio.NewReader(sctx, r)), nil
	case "jsup":
		return sio.NopReadCloser(jsupio.NewReader(sctx, r)), nil
	}
	return nil, fmt.Errorf("no such format: \"%s\"", opts.Format)
}
