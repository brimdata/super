package anyio

import (
	"context"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/arrowio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sio/csupio"
	"github.com/brimdata/super/sio/csvio"
	"github.com/brimdata/super/sio/fjsonio"
	"github.com/brimdata/super/sio/jsonio"
	"github.com/brimdata/super/sio/lineio"
	"github.com/brimdata/super/sio/parquetio"
	"github.com/brimdata/super/sio/supio"
	"github.com/brimdata/super/sio/zeekio"
	"github.com/brimdata/super/vector/vio"
)

func lookupReader(ctx context.Context, sctx *super.Context, r io.Reader, opts ReaderOpts) (vio.Puller, error) {
	switch opts.Format {
	case "arrows":
		r, err := arrowio.NewReader(sctx, r)
		if err != nil {
			return nil, err
		}
		return newVioPuller(sctx, r), nil
	case "bsup":
		scanner, err := bsupio.NewReaderWithOpts(sctx, r, opts.BSUP).NewScanner(ctx, opts.Pushdown)
		if err != nil {
			return nil, err
		}
		return sbuf.NewDematerializer(sctx, scanner), nil
	case "csup":
		return csupio.NewVectorReader(ctx, sctx, r, opts.Pushdown, opts.ConcurrentReaders)
	case "csv":
		return newVioPuller(sctx, csvio.NewReader(sctx, r, opts.CSV)), nil
	case "line":
		return newVioPuller(sctx, lineio.NewReader(r)), nil
	case "fjson":
		return fjsonio.NewVectorReader(context.Background(), sctx, r, opts.Pushdown, opts.ConcurrentReaders), nil
	case "json":
		return newVioPuller(sctx, jsonio.NewReader(sctx, r)), nil
	case "parquet":
		return parquetio.NewVectorReader(ctx, sctx, r, opts.Pushdown, opts.ConcurrentReaders)
	case "sup":
		return newVioPuller(sctx, supio.NewReader(sctx, r)), nil
	case "tsv":
		opts.CSV.Delim = '\t'
		return newVioPuller(sctx, csvio.NewReader(sctx, r, opts.CSV)), nil
	case "zeek":
		return newVioPuller(sctx, zeekio.NewReader(sctx, r)), nil
	}
	return nil, fmt.Errorf("no such format: \"%s\"", opts.Format)
}

func newVioPuller(sctx *super.Context, r sio.Reader) vio.Puller {
	return sbuf.NewDematerializer(sctx, sbuf.NewPuller(r))
}
