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
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/anyio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sio/emitter"
)

type Flags struct {
	anyio.WriterOpts
	DefaultFormat string
	color         bool
	forceBinary   bool
	jsonPretty    bool
	jsonShortcut  bool
	outputFile    string
	pretty        int
	split         string
	splitSize     auto.Bytes
	supPersist    string
	supPretty     bool
	supShortcut   bool
	unbuffered    bool
}

func (f *Flags) Options() anyio.WriterOpts {
	return f.WriterOpts
}

func (f *Flags) setFlags(fs *flag.FlagSet) {
	f.BSUP = &bsupio.WriterOpts{}
	fs.BoolVar(&f.BSUP.Compress, "bsup.compress", true, "compress Super Binary frames")
	fs.IntVar(&f.BSUP.FrameThresh, "bsup.framethresh", bsupio.DefaultFrameThresh,
		"minimum Super Binary frame size in uncompressed bytes")
	fs.BoolVar(&f.color, "color", true, "enable/disable color formatting for -S and db text output")
	fs.StringVar(&f.supPersist, "persist", "",
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
	fs.StringVar(&f.Format, "f", f.DefaultFormat, "format for output data [arrows,bsup,csup,csv,db,json,jsup,line,parquet,sup,table,text,tsv,zeek]")
	fs.BoolVar(&f.forceBinary, "B", false, "allow Super Binary to be sent to a terminal output")
	fs.BoolVar(&f.jsonPretty, "J", false, "use formatted JSON output independent of -f option")
	fs.BoolVar(&f.jsonShortcut, "j", false, "use line-oriented JSON output independent of -f option")
	fs.BoolVar(&f.supPretty, "S", false, "use formatted Super JSON output independent of -f option")
	fs.BoolVar(&f.supShortcut, "s", false, "use line-oriented Super JSON output independent of -f option")
}

func (f *Flags) Init() error {
	f.JSON.Pretty, f.SUP.Pretty = f.pretty, f.pretty
	if f.supPersist != "" {
		re, err := regexp.Compile(f.supPersist)
		if err != nil {
			return err
		}
		f.SUP.Persist = re
	}
	if f.jsonShortcut || f.jsonPretty {
		if f.Format != f.DefaultFormat || f.supShortcut || f.supPretty {
			return errors.New("cannot use -j or -J with -f, -s, or -S")
		}
		f.Format = "json"
		if !f.jsonPretty {
			f.JSON.Pretty = 0
		}
	} else if f.supShortcut || f.supPretty {
		if f.Format != f.DefaultFormat {
			return errors.New("cannot use -s or -S with -f")
		}
		f.Format = "sup"
		if !f.supPretty {
			f.SUP.Pretty = 0
		}
	}
	if f.outputFile == "-" {
		f.outputFile = ""
	}
	if f.outputFile == "" && f.split == "" && f.Format == "bsup" && !f.forceBinary &&
		terminal.IsTerminalFile(os.Stdout) {
		f.Format = "sup"
		f.SUP.Pretty = 0
	}
	if f.unbuffered {
		sbuf.PullerBatchValues = 1
	}
	return nil
}

func (f *Flags) FileName() string {
	return f.outputFile
}

func (f *Flags) Open(ctx context.Context, engine storage.Engine) (sio.WriteCloser, error) {
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
