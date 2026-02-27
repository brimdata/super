package root

import (
	"errors"
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
	"github.com/brimdata/super/compiler/sfmt"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/supio"
)

var Super = &charm.Spec{
	Name:        "super",
	Usage:       "super [options] <command> | super [ options ] [ -c query ] [ file ... ]",
	Short:       "process data with SuperSQL queries",
	HiddenFlags: "cpuprofile,memprofile,trace",
	Long: `
See https://superdb.org/command/super.html
`,
	New:          New,
	InternalLeaf: true,
}

type Command struct {
	// common flags
	cli.Flags
	// query runtime flags
	canon        bool
	stopErr      bool
	inputFlags   inputflags.Flags
	outputFlags  outputflags.Flags
	queryFlags   queryflags.Flags
	runtimeFlags runtimeflags.Flags
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
}

func (c *Command) Run(args []string) error {
	if c.canon && len(c.queryFlags.Query) == 0 {
		return errors.New("query text must be specified (-c or -I) when using -C")
	}
	ctx, cleanup, err := c.Init(&c.inputFlags, &c.outputFlags, &c.runtimeFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) == 0 && len(c.queryFlags.Query) == 0 {
		return charm.NeedHelp
	}
	ast, err := parser.ParseFiles(c.queryFlags.Query)
	if err != nil {
		return err
	}
	if c.canon {
		fmt.Println(sfmt.AST(ast.Parsed()))
		return nil
	}
	if len(args) > 0 {
		ast.PrependFileScan(args)
	}
	env := exec.NewEnvironment(storage.NewLocalEngine(), nil)
	env.Dynamic = c.inputFlags.Dynamic
	env.IgnoreOpenErrors = !c.stopErr
	env.ReaderOpts = c.inputFlags.ReaderOpts
	env.Runtime = c.runtimeFlags.Runtime
	env.SampleSize = c.inputFlags.SampleSize
	comp := compiler.NewCompilerWithEnv(env)
	query, err := runtime.CompileQuery(ctx, super.NewContext(), comp, ast, nil)
	if err != nil {
		return err
	}
	defer query.Pull(true)
	writer, err := c.outputFlags.Open(ctx, env.Engine())
	if err != nil {
		return err
	}
	out := map[string]sio.WriteCloser{
		"main":  writer,
		"debug": supio.NewWriter(sio.NopCloser(os.Stderr), supio.WriterOpts{}),
	}
	err = sbuf.CopyMux(out, query)
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	c.queryFlags.PrintStats(query.Progress())
	return err
}
