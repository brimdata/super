package outputflags

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"

	"github.com/brimdata/super/cli/auto"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/pkg/terminal"
	"github.com/brimdata/super/pkg/terminal/color"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/anyio"
	"github.com/brimdata/super/zio/emitter"
	"github.com/brimdata/super/zio/zngio"
)

type Flags struct {
	anyio.WriterOpts
	DefaultFormat string
	color         bool
	forceBinary   bool
	jsonPretty    bool
	jsonShortcut  bool
	jsupPersist   string
	jsupPretty    bool
	jsupShortcut  bool
	outputFile    string
	pretty        int
	split         string
	splitSize     auto.Bytes
	unbuffered    bool
}

func (f *Flags) Options() anyio.WriterOpts {
	return f.WriterOpts
}

func (f *Flags) setFlags(fs *flag.FlagSet) {
	f.ZNG = &zngio.WriterOpts{}
	fs.BoolVar(&f.ZNG.Compress, "bsup.compress", true, "compress Super Binary frames")
	fs.IntVar(&f.ZNG.FrameThresh, "bsup.framethresh", zngio.DefaultFrameThresh,
		"minimum Super Binary frame size in uncompressed bytes")
	fs.BoolVar(&f.color, "color", true, "enable/disable color formatting for -Z and lake text output")
	fs.StringVar(&f.jsupPersist, "persist", "",
		"regular expression to persist type definitions across the stream")
	fs.IntVar(&f.pretty, "pretty", 4,
		"tab size to pretty print JSON and Super JSON output (0 for newline-delimited output")
	fs.StringVar(&f.outputFile, "o", "", "write data to output file")
	fs.StringVar(&f.split, "split", "",
		"split output into one file per data type in this directory (but see -splitsize)")
	fs.Var(&f.splitSize, "splitsize",
		"if >0 and -split is set, split into files at least this big rather than by data type")
	fs.BoolVar(&f.unbuffered, "unbuffered", false, "disable output buffering")
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	f.SetFormatFlags(fs)
	f.setFlags(fs)
}

func (f *Flags) SetFlagsWithFormat(fs *flag.FlagSet, format string) {
	f.setFlags(fs)
	f.Format = format
}

func (f *Flags) SetFormatFlags(fs *flag.FlagSet) {
	if f.DefaultFormat == "" {
		f.DefaultFormat = "bsup"
	}
	fs.StringVar(&f.Format, "f", f.DefaultFormat, "format for output data [arrows,bsup,csup,csv,json,jsup,lake,line,parquet,table,text,tsv,zeek,zjson]")
	fs.BoolVar(&f.forceBinary, "B", false, "allow Super Binary to be sent to a terminal output")
	fs.BoolVar(&f.jsonPretty, "J", false, "use formatted JSON output independent of -f option")
	fs.BoolVar(&f.jsonShortcut, "j", false, "use line-oriented JSON output independent of -f option")
	fs.BoolVar(&f.jsupPretty, "Z", false, "use formatted Super JSON output independent of -f option")
	fs.BoolVar(&f.jsupShortcut, "z", false, "use line-oriented Super JSON output independent of -f option")
}

func (f *Flags) Init() error {
	f.JSON.Pretty, f.ZSON.Pretty = f.pretty, f.pretty
	if f.jsupPersist != "" {
		re, err := regexp.Compile(f.jsupPersist)
		if err != nil {
			return err
		}
		f.ZSON.Persist = re
	}
	if f.jsonShortcut || f.jsonPretty {
		if f.Format != f.DefaultFormat || f.jsupShortcut || f.jsupPretty {
			return errors.New("cannot use -j or -J with -f, -z, or -Z")
		}
		f.Format = "json"
		if !f.jsonPretty {
			f.JSON.Pretty = 0
		}
	} else if f.jsupShortcut || f.jsupPretty {
		if f.Format != f.DefaultFormat {
			return errors.New("cannot use -z or -Z with -f")
		}
		f.Format = "jsup"
		if !f.jsupPretty {
			f.ZSON.Pretty = 0
		}
	}
	if f.outputFile == "-" {
		f.outputFile = ""
	}
	if f.outputFile == "" && f.split == "" && f.Format == "bsup" && !f.forceBinary &&
		terminal.IsTerminalFile(os.Stdout) {
		f.Format = "jsup"
		f.ZSON.Pretty = 0
	}
	if f.unbuffered {
		zbuf.PullerBatchValues = 1
	}
	return nil
}

func (f *Flags) FileName() string {
	return f.outputFile
}

func (f *Flags) Open(ctx context.Context, engine storage.Engine) (zio.WriteCloser, error) {
	if f.split != "" {
		dir, err := storage.ParseURI(f.split)
		if err != nil {
			return nil, fmt.Errorf("-split option: %w", err)
		}
		if size := f.splitSize.Bytes; size > 0 {
			return emitter.NewSizeSplitter(ctx, engine, dir, f.outputFile, f.unbuffered, f.WriterOpts, int64(size))
		}
		d, err := emitter.NewSplit(ctx, engine, dir, f.outputFile, f.unbuffered, f.WriterOpts)
		if err != nil {
			return nil, err
		}
		return d, nil
	}
	if f.outputFile == "" && f.color && terminal.IsTerminalFile(os.Stdout) {
		color.Enabled = true
	}
	w, err := emitter.NewFileFromPath(ctx, engine, f.outputFile, f.unbuffered, f.WriterOpts)
	if err != nil {
		return nil, err
	}
	return w, nil
}
