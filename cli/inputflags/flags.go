package inputflags

import (
	"errors"
	"flag"

	"github.com/brimdata/super/cli/auto"
	"github.com/brimdata/super/sio/anyio"
	"github.com/brimdata/super/sio/bsupio"
)

type Flags struct {
	Dynamic      bool
	ReaderOpts   anyio.ReaderOpts
	bsupReadMax  auto.Bytes
	bsupReadSize auto.Bytes
}

func (f *Flags) SetFlags(fs *flag.FlagSet, validate bool) {
	f.bsupReadMax = auto.NewBytes(bsupio.MaxSize)
	fs.Var(&f.bsupReadMax, "bsup.readmax", "maximum Super Binary read buffer size in MiB, MB, etc.")
	f.bsupReadSize = auto.NewBytes(bsupio.ReadSize)
	fs.Var(&f.bsupReadSize, "bsup.readsize", "target Super Binary read buffer size in MiB, MB, etc.")
	opts := &f.ReaderOpts
	fs.IntVar(&opts.BSUP.Threads, "bsup.threads", 0, "number of Super Binary read threads (0=GOMAXPROCS)")
	fs.BoolVar(&opts.BSUP.Validate, "bsup.validate", validate, "validate format when reading Super Binary")
	opts.CSV.Delim = ','
	fs.Func("csv.delim", `CSV field delimiter (default ",")`, func(s string) error {
		if len(s) != 1 {
			return errors.New("CSV field delimiter must be exactly one character")
		}
		opts.CSV.Delim = rune(s[0])
		return nil

	})
	fs.BoolVar(&f.Dynamic, "dynamic", false, "disable static type checking of inputs")
	fs.StringVar(&opts.Format, "i", "auto", "format of input data [auto,arrows,bsup,csup,csv,json,jsup,line,parquet,sup,tsv,zeek]")
}

// Init is called after flags have been parsed.
func (f *Flags) Init() error {
	bsup := &f.ReaderOpts.BSUP
	bsup.Max = int(f.bsupReadMax.Bytes)
	if bsup.Max < 0 {
		return errors.New("max read buffer size must be greater than zero")
	}
	bsup.Size = int(f.bsupReadSize.Bytes)
	if bsup.Size < 0 {
		return errors.New("target read buffer size must be greater than zero")
	}
	return nil
}
