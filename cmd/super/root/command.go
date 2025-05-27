package root

import (
	"flag"
	"fmt"
	"os"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli"
	"github.com/brimdata/super/cli/inputflags"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cli/queryflags"
	"github.com/brimdata/super/cli/runtimeflags"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zfmt"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/supio"
)

var Super = &charm.Spec{
	Name:        "super",
	Usage:       "super [options] <command> | super [ options ] [ -c query ] [ file ... ]",
	Short:       "process data with SuperSQL queries",
	HiddenFlags: "cpuprofile,memprofile,trace",
	Long: `
The "super" command provides a way to process data in diverse input formats,
providing search, analytics, and extensive transformations using
the SuperSQL query language.
A query typically applies Boolean logic or keyword search to filter
the input and then transforms or analyzes the filtered stream.
Output is written to one or more files or to standard output.

A query is comprised of one or more operators interconnected
into a pipeline using the pipe symbol "|" or the alternate "|>".
See https://zed.brimdata.io/docs/language
for details.  The "select" and "from" operators provide backward
compatibility with SQL. In fact, you can use SQL exclusively and
avoid pipeline operators altogether if you prefer.

Supported file formats include Arrow, CSV, JSON, Parquet,
Super JSON, Super Binary, Super Columnar, and Zeek TSV.

Input files may be file system paths;
"-" for standard input; or HTTP, HTTPS, or S3 URLs.
For most types of data, the input format is automatically detected.
If multiple files are specified, each file format is determined independently
so you can mix and match input types.  If multiple files are concatenated
into a stream and presented as standard input, the files must all be of the
same type as the beginning of the stream will determine the format.

If no input file is specified, the default of a single null input value will be
fed to the query.  This is analogous to SQL's default input of a single
empty input row.

Output is sent to standard output unless an output file is specified with -o.
Some output formats like Parquet are based on schemas and require all
data in the output to conform to the same schema.  To handle this, you can
either fuse the data into a union of all the record types present
(presuming all the output values are records) or you can specify the -split
flag to indicate a destination directory for separate output files for each
output type.  This flag may be used in combination with -o, which
provides the prefix for the file path, e.g.,

  super -f parquet -split out -o example-output input.bsup

When writing to stdout and stdout is a terminal, the default output format is Super JSON.
Otherwise, the default format is Super Binary.  In either case, the default
may be overridden with -f, -s, or -S.

The query text may include source files using -I, which is particularly
convenient when a large, complex query spans multiple lines.  In this case,
these source files are concatenated together along with the command-line query text
in the order appearing on the command line.  Any error messages are properly
collated to the included file in which they occurred.

The runtime processes input natively as super-structured data so if you intend to run
many queries over the same data, you will see substantial performance gains
by converting your data to the Super Binary format, e.g.,

  super -f bsup input.any > fast.bsup

  super -c <query> fast.bsup

Please see https://github.com/brimdata/super for more information.
`,
	New:          New,
	InternalLeaf: true,
}

type Command struct {
	// common flags
	cli.Flags
	// query runtime flags
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
	c := &Command{}
	c.SetFlags(f)
	return c, nil
}

func (c *Command) SetLeafFlags(f *flag.FlagSet) {
	c.outputFlags.SetFlags(f)
	c.inputFlags.SetFlags(f, false)
	c.queryFlags.SetFlags(f)
	c.runtimeFlags.SetFlags(f)
	f.BoolVar(&c.canon, "C", false, "display parsed AST in a textual format")
	f.BoolVar(&c.stopErr, "e", true, "stop upon input errors")
	f.BoolVar(&c.quiet, "q", false, "don't display warnings")
	f.StringVar(&c.query, "c", "", "query to execute")
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
	ast, err := parser.ParseQuery(c.query, c.queryFlags.Includes...)
	if err != nil {
		return err
	}
	if c.canon {
		fmt.Println(zfmt.AST(ast.Parsed()))
		return nil
	}
	sctx := super.NewContext()
	local := storage.NewLocalEngine()
	var readers []zio.Reader
	if len(args) > 0 {
		readers, err = c.inputFlags.Open(ctx, sctx, local, args, c.stopErr)
		if err != nil {
			return err
		}
		defer zio.CloseReaders(readers)
	}
	writer, err := c.outputFlags.Open(ctx, local)
	if err != nil {
		return err
	}
	comp := compiler.NewCompiler(local)
	query, err := runtime.CompileQuery(ctx, sctx, comp, ast, readers)
	if err != nil {
		return err
	}
	defer query.Pull(true)
	out := map[string]zio.WriteCloser{
		"main":  writer,
		"debug": supio.NewWriter(zio.NopCloser(os.Stderr), supio.WriterOpts{}),
	}
	err = zbuf.CopyMux(out, query)
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	c.queryFlags.PrintStats(query.Progress())
	return err
}
