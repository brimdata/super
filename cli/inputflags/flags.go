package inputflags

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/brimdata/zed/cli/auto"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/anyio"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zson"
)

type Flags struct {
	anyio.ReaderOpts
	ReadMax  auto.Bytes
	ReadSize auto.Bytes
}

func (f *Flags) Options() anyio.ReaderOpts {
	return f.ReaderOpts
}

func (f *Flags) SetFlags(fs *flag.FlagSet, validate bool) {
	fs.StringVar(&f.Format, "i", "auto", "format of input data [auto,zng,zst,json,ndjson,zeek,zjson,csv,tzng,parquet]")
	fs.BoolVar(&f.Zng.Validate, "validate", validate, "validate the input format when reading ZNG streams")
	f.ReadMax = auto.NewBytes(zngio.MaxSize)
	fs.Var(&f.ReadMax, "readmax", "maximum memory used read buffers in MiB, MB, etc")
	f.ReadSize = auto.NewBytes(zngio.ReadSize)
	fs.Var(&f.ReadSize, "readsize", "target memory used read buffers in MiB, MB, etc")
}

// Init is called after flags have been parsed.
func (f *Flags) Init() error {
	f.Zng.Max = int(f.ReadMax.Bytes)
	if f.Zng.Max < 0 {
		return errors.New("max read buffer size must be greater than zero")
	}
	f.Zng.Size = int(f.ReadSize.Bytes)
	if f.Zng.Size < 0 {
		return errors.New("target read buffer size must be greater than zero")
	}
	return nil
}

func (f *Flags) Open(zctx *zson.Context, engine storage.Engine, paths []string, stopOnErr bool) ([]zio.Reader, error) {
	var readers []zio.Reader
	for _, path := range paths {
		if path == "-" {
			path = "stdio:stdin"
		}
		file, err := anyio.OpenFile(zctx, engine, path, f.ReaderOpts)
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
