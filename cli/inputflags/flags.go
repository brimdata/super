package inputflags

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/auto"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/anyio"
	"github.com/brimdata/super/sio/bsupio"
)

type Flags struct {
	anyio.ReaderOpts
	ReadMax  auto.Bytes
	ReadSize auto.Bytes
	Threads  int
}

func (f *Flags) Options() anyio.ReaderOpts {
	return f.ReaderOpts
}

func (f *Flags) SetFlags(fs *flag.FlagSet, validate bool) {
	fs.StringVar(&f.Format, "i", "auto", "format of input data [auto,arrows,bsup,csup,csv,json,line,parquet,sup,tsv,zeek,jsup]")
	f.CSV.Delim = ','
	fs.Func("csv.delim", `CSV field delimiter (default ",")`, func(s string) error {
		if len(s) != 1 {
			return errors.New("CSV field delimiter must be exactly one character")
		}
		f.CSV.Delim = rune(s[0])
		return nil

	})
	fs.IntVar(&f.BSUP.Threads, "bsup.threads", 0, "number of Super Binary read threads (0=GOMAXPROCS)")
	fs.BoolVar(&f.BSUP.Validate, "bsup.validate", validate, "validate format when reading Super Binary")
	f.ReadMax = auto.NewBytes(bsupio.MaxSize)
	fs.Var(&f.ReadMax, "bsup.readmax", "maximum Super Binary read buffer size in MiB, MB, etc.")
	f.ReadSize = auto.NewBytes(bsupio.ReadSize)
	fs.Var(&f.ReadSize, "bsup.readsize", "target Super Binary read buffer size in MiB, MB, etc.")
}

// Init is called after flags have been parsed.
func (f *Flags) Init() error {
	f.BSUP.Max = int(f.ReadMax.Bytes)
	if f.BSUP.Max < 0 {
		return errors.New("max read buffer size must be greater than zero")
	}
	f.BSUP.Size = int(f.ReadSize.Bytes)
	if f.BSUP.Size < 0 {
		return errors.New("target read buffer size must be greater than zero")
	}
	return nil
}

func (f *Flags) Open(ctx context.Context, sctx *super.Context, engine storage.Engine, paths []string, stopOnErr bool) ([]sio.Reader, error) {
	var readers []sio.Reader
	for _, path := range paths {
		if path == "-" {
			path = "stdio:stdin"
		}
		file, err := anyio.Open(ctx, sctx, engine, path, f.ReaderOpts)
		if err != nil {
			err = fmt.Errorf("%s: %w", path, err)
			if stopOnErr {
				return nil, err
			}
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		readers = append(readers, file)
	}
	return readers, nil
}
