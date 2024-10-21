package query

import (
	"flag"
	"fmt"
	"os"

	 "github.com/brimdata/super"
	"github.com/brimdata/super/cli/inputflags"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cli/queryflags"
	"github.com/brimdata/super/cli/runtimeflags"
	"github.com/brimdata/super/cmd/super/root"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zfmt"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/zsonio"
)

var spec = &charm.Spec{
	Name:        "query",
	Usage:       "query [ options ] [ zed-query ] file [ file ... ]",
	Short:       "process data with Zed queries",
	HiddenFlags: "cpuprofile,memprofile,pathregexp",
	Long: `
XXX "zq" is a command-line tool for processing data in diverse input formats,
providing search, analytics, and extensive transormations using
the Zed query language.
A query typically applies Boolean logic or keyword search to filter
the input and then transforms or analyzes the filtered stream.
Output is written to one or more files or to standard output.

A Zed query is comprised of one or more operators interconnected
into a pipeline using the Unix pipe character "|".
See https://github.com/brimdata/super/tree/main/docs/language
for details.

Supported input formats include CSV, JSON, NDJSON, Parquet,
VNG, ZNG, ZSON, and Zeek TSV.  Supported output formats include
all the input formats along with a SQL-like table format.

"zq" must be run with at least one input.  Input files can
be file system paths; "-" for standard input; or HTTP, HTTPS, or S3 URLs.
For most types of data, the input format is automatically detected.
If multiple files are specified, each file format is determined independently
so you can mix and match input types.  If multiple files are concatenated
into a stream and presented as standard input, the files must all be of the
same type as the beginning of stream will determine the format.

Output is sent to standard output unless an output file is specified with -o.
Some output formats like Parquet are based on schemas and require all
data in the output to conform to the same schema.  To handle this, you can
either fuse the data into a union of all the record types present
(presuming all the output values are records) or you can specify the -split
flag to indicate a destination directory for separate output files for each
output type.  This flag may be used in combination with -o, which
provides the prefix for the file path, e.g.,

  zq -f parquet -split out -o example-output input.zng

When writing to stdout and stdout is a terminal, the default output format is ZSON.
Otherwise, the default format is binary ZNG.  In either case, the default
may be overridden with -f, -z, or -Z.

After the options, a Zed "query" string may be specified as a
single argument conforming to the Zed language syntax;
i.e., it should be quoted as a single string in the shell.

If the first argument is a path to a valid file rather than a Zed query,
then the Zed query is assumed to be "*", i.e., match and output all
of the input.  If the first argument is both a valid Zed query
and an existing file, then the file overrides.

The Zed query text may include source files using -I, which is particularly
convenient when a large, complex query spans multiple lines.  In this case,
these source files are concatenated together along with the command-line query text
in the order appearing on the command line.

The "zq" engine processes data natively in Zed so if you intend to run
many queries over the same data, you will see substantial performance gains
by converting your data to the efficient binary form of Zed called ZNG, e.g.,

  zq -f zng input.json > fast.zng
  zq <query> fast.zng
  ...

Please see https://github.com/brimdata/super for more information.
`,
	New: New,
}

func init() {
	root.Super.Add(spec)
}

type Command struct {
	*root.Command
	canon        bool
	quiet        bool
	stopErr      bool
	inputFlags   inputflags.Flags
	outputFlags  outputflags.Flags
	queryFlags   queryflags.Flags
	runtimeFlags runtimeflags.Flags
	query        string
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.outputFlags.SetFlags(f)
	c.inputFlags.SetFlags(f, false)
	c.queryFlags.SetFlags(f)
	c.runtimeFlags.SetFlags(f)
	f.BoolVar(&c.canon, "C", false, "display AST in Zed canonical format")
	f.BoolVar(&c.stopErr, "e", true, "stop upon input errors")
	f.BoolVar(&c.quiet, "q", false, "don't display warnings")
	f.StringVar(&c.query, "c", "", "query to execute")
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.inputFlags, &c.outputFlags, &c.runtimeFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) == 0 && len(c.queryFlags.Includes) == 0 && c.query == "" {
		return charm.NeedHelp
	}
	if c.canon {
		flowgraph, _, err := parser.ParseSuperPipe(c.queryFlags.Includes, c.query)
		if err != nil {
			return err
		}
		fmt.Println(zfmt.AST(flowgraph))
		return nil
	}
	paths, flowgraph, sset, null, err := c.queryFlags.ParseSourcesAndInputs(c.query, args)
	if err != nil {
		return fmt.Errorf("super query: %w", err)
	}
	zctx := super.NewContext()
	local := storage.NewLocalEngine()
	var readers []zio.Reader
	if null {
		readers = []zio.Reader{zbuf.NewArray([]super.Value{super.Null})}
	} else {
		readers, err = c.inputFlags.Open(ctx, zctx, local, paths, c.stopErr)
		if err != nil {
			return err
		}
	}
	defer zio.CloseReaders(readers)
	writer, err := c.outputFlags.Open(ctx, local)
	if err != nil {
		return err
	}
	comp := compiler.NewFileSystemCompiler(local)
	query, err := runtime.CompileQuery(ctx, zctx, comp, flowgraph, sset, readers)
	if err != nil {
		return err
	}
	defer query.Pull(true)
	out := map[string]zio.WriteCloser{
		"main":  writer,
		"debug": zsonio.NewWriter(zio.NopCloser(os.Stderr), zsonio.WriterOpts{}),
	}
	err = zbuf.CopyMux(out, query)
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	c.queryFlags.PrintStats(query.Progress())
	return err
}
