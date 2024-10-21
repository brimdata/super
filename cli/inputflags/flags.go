package inputflags

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/auto"
	"github.com/brimdata/super/compiler/optimizer/demand"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/anyio"
	"github.com/brimdata/super/zio/zngio"
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
	fs.StringVar(&f.Format, "i", "auto", "format of input data [auto,arrows,csv,json,line,parquet,tsv,vng,zeek,zjson,zng,zson]")
	f.CSV.Delim = ','
	fs.Func("csv.delim", `CSV field delimiter (default ",")`, func(s string) error {
		if len(s) != 1 {
			return errors.New("CSV field delimiter must be exactly one character")
		}
		f.CSV.Delim = rune(s[0])
		return nil

	})
	fs.BoolVar(&f.ZNG.Validate, "zng.validate", validate, "validate format when reading ZNG")
	fs.IntVar(&f.ZNG.Threads, "zng.threads", 0, "number of ZNG read threads (0=GOMAXPROCS)")
	f.ReadMax = auto.NewBytes(zngio.MaxSize)
	fs.Var(&f.ReadMax, "zng.readmax", "maximum ZNG read buffer size in MiB, MB, etc.")
	f.ReadSize = auto.NewBytes(zngio.ReadSize)
	fs.Var(&f.ReadSize, "zng.readsize", "target ZNG read buffer size in MiB, MB, etc.")
}

// Init is called after flags have been parsed.
func (f *Flags) Init() error {
	f.ZNG.Max = int(f.ReadMax.Bytes)
	if f.ZNG.Max < 0 {
		return errors.New("max read buffer size must be greater than zero")
	}
	f.ZNG.Size = int(f.ReadSize.Bytes)
	if f.ZNG.Size < 0 {
		return errors.New("target read buffer size must be greater than zero")
	}
	return nil
}

func (f *Flags) Open(ctx context.Context, zctx *super.Context, engine storage.Engine, paths []string, stopOnErr bool) ([]zio.Reader, error) {
	var readers []zio.Reader
	for _, path := range paths {
		if path == "-" {
			path = "stdio:stdin"
		}
		file, err := anyio.Open(ctx, zctx, engine, path, demand.All(), f.ReaderOpts)
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
