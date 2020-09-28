package outputflags

import (
	"errors"
	"flag"
	"os"

	"github.com/brimsec/zq/emitter"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio"
	"github.com/brimsec/zq/zio/zngio"
	"github.com/brimsec/zq/zio/zstio"
	"golang.org/x/crypto/ssh/terminal"
)

type Flags struct {
	zio.WriterOpts
	dir          string
	outputFile   string
	forceBinary  bool
	textShortcut bool
}

func (f *Flags) Options() zio.WriterOpts {
	return f.WriterOpts
}

func (f *Flags) setFlags(fs *flag.FlagSet) {
	// zio stuff
	fs.BoolVar(&f.UTF8, "U", false, "display zeek strings as UTF-8")
	fs.BoolVar(&f.Text.ShowTypes, "T", false, "display field types in text output")
	fs.BoolVar(&f.Text.ShowFields, "F", false, "display field names in text output")
	fs.BoolVar(&f.EpochDates, "E", false, "display epoch timestamps in csv and text output")
	fs.IntVar(&f.Zng.StreamRecordsMax, "b", 0, "limit for number of records in each ZNG stream (0 for no limit)")
	fs.IntVar(&f.Zng.LZ4BlockSize, "znglz4blocksize", zngio.DefaultLZ4BlockSize,
		"LZ4 block size in bytes for ZNG compression (nonpositive to disable)")
	f.Zst.ColumnThresh = zstio.DefaultColumnThresh
	fs.Var(&f.Zst.ColumnThresh, "coltresh", "minimum frame size (MiB) used for zst columns")
	f.Zst.SkewThresh = zstio.DefaultSkewThresh
	fs.Var(&f.Zst.SkewThresh, "skewtresh", "minimum skew size (MiB) used to group zst columns")

	// emitter stuff
	fs.StringVar(&f.dir, "d", "", "directory for output data files")
	fs.StringVar(&f.outputFile, "o", "", "write data to output file")
	fs.BoolVar(&f.forceBinary, "B", false, "allow binary zng be sent to a terminal output")

}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	f.setFlags(fs)
	fs.StringVar(&f.Format, "f", "zng", "format for output data [zng,zst,ndjson,table,text,csv,zeek,zjson,tzng]")
	fs.BoolVar(&f.textShortcut, "t", false, "use format tzng independent of -f option")
}

func (f *Flags) SetFlagsWithFormat(fs *flag.FlagSet, format string) {
	f.setFlags(fs)
	f.Format = format
}

func (f *Flags) Init() error {
	if f.textShortcut {
		if f.Format != "zng" {
			return errors.New("cannot use -t with -f")
		}
		f.Format = "tzng"
	}
	if f.outputFile == "-" {
		f.outputFile = ""
	}
	if f.outputFile == "" && f.Format == "zng" && IsTerminal(os.Stdout) && !f.forceBinary {
		return errors.New("writing binary zng data to terminal; override with -B or use -t for text.")
	}
	return nil
}

func (f *Flags) InitWithFormat(format string) error {
	if f.outputFile == "-" {
		f.outputFile = ""
	}
	if f.outputFile == "" && f.Format == "zng" && IsTerminal(os.Stdout) && !f.forceBinary {
		return errors.New("writing binary zng data to terminal; override with -B or use -t for text.")
	}
	return nil
}

func (f *Flags) FileName() string {
	return f.outputFile
}

func (f *Flags) Open() (zbuf.WriteCloser, error) {
	if f.dir != "" {
		d, err := emitter.NewDir(f.dir, f.outputFile, os.Stderr, f.WriterOpts)
		if err != nil {
			return nil, err
		}
		return d, nil
	}
	w, err := emitter.NewFile(f.outputFile, f.WriterOpts)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func IsTerminal(f *os.File) bool {
	return terminal.IsTerminal(int(f.Fd()))
}
