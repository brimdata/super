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
	split         string
	splitSize     auto.Bytes
	outputFile    string
	forceBinary   bool
	jsonShortcut  bool
	jsonPretty    bool
	zsonShortcut  bool
	zsonPretty    bool
	zsonPersist   string
	color         bool
	pretty        int
	unbuffered    bool
}

func (f *Flags) Options() anyio.WriterOpts {
	return f.WriterOpts
}

func (f *Flags) setFlags(fs *flag.FlagSet) {
	// zio stuff
	fs.BoolVar(&f.color, "color", true, "enable/disable color formatting for -Z and lake text output")
	f.ZNG = &zngio.WriterOpts{}
	fs.BoolVar(&f.ZNG.Compress, "zng.compress", true, "compress ZNG frames")
	fs.IntVar(&f.ZNG.FrameThresh, "zng.framethresh", zngio.DefaultFrameThresh,
		"minimum ZNG frame size in uncompressed bytes")
	fs.IntVar(&f.pretty, "pretty", 4,
		"tab size to pretty print JSON/ZSON output (0 for newline-delimited JSON/ZSON")
	fs.StringVar(&f.zsonPersist, "persist", "",
		"regular expression to persist type definitions across the stream")

	// emitter stuff
	fs.StringVar(&f.split, "split", "",
		"split output into one file per data type in this directory (but see -splitsize)")
	fs.Var(&f.splitSize, "splitsize",
		"if >0 and -split is set, split into files at least this big rather than by data type")
	fs.BoolVar(&f.unbuffered, "unbuffered", false, "disable output buffering")
	fs.StringVar(&f.outputFile, "o", "", "write data to output file")
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
		f.DefaultFormat = "zng"
	}
	fs.StringVar(&f.Format, "f", f.DefaultFormat, "format for output data [arrows,csv,json,lake,parquet,table,text,tsv,vng,zeek,zjson,zng,zson]")
	fs.BoolVar(&f.jsonShortcut, "j", false, "use line-oriented JSON output independent of -f option")
	fs.BoolVar(&f.jsonPretty, "J", false, "use formatted JSON output independent of -f option")
	fs.BoolVar(&f.zsonShortcut, "z", false, "use line-oriented ZSON output independent of -f option")
	fs.BoolVar(&f.zsonPretty, "Z", false, "use formatted ZSON output independent of -f option")
	fs.BoolVar(&f.forceBinary, "B", false, "allow binary zng be sent to a terminal output")
}

func (f *Flags) Init() error {
	f.JSON.Pretty, f.ZSON.Pretty = f.pretty, f.pretty
	if f.zsonPersist != "" {
		re, err := regexp.Compile(f.zsonPersist)
		if err != nil {
			return err
		}
		f.ZSON.Persist = re
	}
	if f.jsonShortcut || f.jsonPretty {
		if f.Format != f.DefaultFormat || f.zsonShortcut || f.zsonPretty {
			return errors.New("cannot use -j or -J with -f, -z, or -Z")
		}
		f.Format = "json"
		if !f.jsonPretty {
			f.JSON.Pretty = 0
		}
	} else if f.zsonShortcut || f.zsonPretty {
		if f.Format != f.DefaultFormat {
			return errors.New("cannot use -z or -Z with -f")
		}
		f.Format = "zson"
		if !f.zsonPretty {
			f.ZSON.Pretty = 0
		}
	}
	if f.outputFile == "-" {
		f.outputFile = ""
	}
	if f.outputFile == "" && f.split == "" && f.Format == "zng" && !f.forceBinary &&
		terminal.IsTerminalFile(os.Stdout) {
		f.Format = "zson"
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
