package zq

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brimsec/zq/archive"
	"github.com/brimsec/zq/ast"
	"github.com/brimsec/zq/cmd/zar/root"
	"github.com/brimsec/zq/driver"
	"github.com/brimsec/zq/emitter"
	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio"
	"github.com/brimsec/zq/zio/detector"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/brimsec/zq/zql"
	"github.com/mccanne/charm"
	"go.uber.org/zap"
)

var Zq = &charm.Spec{
	Name:  "zq",
	Usage: "zq [-R dir] [options] [zql] file [file...]",
	Short: "walk an archive and run zql queries",
	Long: `
"zar zq" descends the directory given by the -R option (or ZAR_ROOT env) looking for
logs with zar directories and for each such directory found, it runs
the zq logic relative to that directory and emits the results in zng format.
The file names here are relative to that directory and the special name "_" refers
to the actual log file in the parent of the zar directory.

If the root directory is not specified by either the ZAR_ROOT environemnt
variable or the -R option, then the current directory is assumed.
`,
	New: New,
}

func init() {
	root.Zar.Add(Zq)
}

type Command struct {
	*root.Command
	root       string
	outputFile string
	stopErr    bool
	quiet      bool
}

func fileExists(path string) bool {
	if path == "-" {
		return true
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	f.StringVar(&c.root, "R", os.Getenv("ZAR_ROOT"), "root directory of zar archive to walk")
	f.BoolVar(&c.quiet, "q", false, "don't display zql warnings")
	f.StringVar(&c.outputFile, "o", "", "write data to output file")
	f.BoolVar(&c.stopErr, "e", true, "stop upon input errors")

	return c, nil
}

//XXX lots here copied from zq command... we should refactor into a tools package
func (c *Command) Run(args []string) error {
	//XXX
	if c.outputFile == "-" {
		c.outputFile = ""
	}

	ark, err := archive.OpenArchive(c.root, nil)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return errors.New("zar zq needs input arguments")
	}
	// XXX this is parallelizable except for writing to stdout when
	// concatenating results
	return archive.Walk(ark, func(zardir string) error {
		inputs := args
		var query ast.Proc
		var err error
		first := archive.Localize(zardir, inputs[0])
		if first != "" && fileExists(first) {
			query, err = zql.ParseProc("*")
			if err != nil {
				return err
			}
		} else {
			query, err = zql.ParseProc(inputs[0])
			if err != nil {
				return err
			}
			inputs = inputs[1:]
		}
		var localPaths []string
		for _, input := range inputs {
			localPaths = append(localPaths, archive.Localize(zardir, input))
		}
		paths, err := c.verifyPaths(localPaths)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			// skip and warn if no inputs found
			if !c.quiet {
				fmt.Fprintf(os.Stderr, "%s: no inputs files found\n", zardir)
			}
			return nil
		}
		cfg := detector.OpenConfig{Format: "zng"}
		rc := detector.MultiFileReader(resolver.NewContext(), paths, cfg)
		defer rc.Close()
		reader := zbuf.Reader(rc)
		wch := make(chan string, 5)
		if !c.stopErr {
			reader = zbuf.NewWarningReader(reader, wch)
		}
		writer, err := c.openOutput(zardir, c.outputFile)
		if err != nil {
			return err
		}
		defer writer.Close()
		// XXX we shouldn't need zap here, nano?  etc
		reverse := ark.DataSortDirection == zbuf.DirTimeReverse
		mux, err := driver.CompileWarningsCh(context.Background(), query, reader, reverse, nano.MaxSpan, zap.NewNop(), wch)
		if err != nil {
			return err
		}
		d := driver.NewCLI(writer)
		if !c.quiet {
			d.SetWarningsWriter(os.Stderr)
		}
		return driver.Run(mux, d, nil)
	})
}

func (c *Command) verifyPaths(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		stat, err := os.Stat(path)
		if os.IsNotExist(err) {
			if !c.quiet {
				fmt.Fprintf(os.Stderr, "warning: %s not found\n", path)
			}
			continue
		}
		if err == nil && stat.IsDir() {
			err = fmt.Errorf("path is a directory")
		}
		if err != nil {
			err = fmt.Errorf("%s: %w", path, err)
			if c.stopErr {
				return nil, err
			}
			fmt.Fprintf(os.Stderr, "%s\n", err)
			continue
		}
		files = append(files, path)
	}
	return files, nil
}

func (c *Command) openOutput(zardir, filename string) (zbuf.WriteCloser, error) {
	path := filename
	// prepend path if not stdout
	if path != "" {
		path = filepath.Join(zardir, filename)
	}
	w, err := emitter.NewFile(path, &zio.WriterFlags{Format: "zng"})
	if err != nil {
		return nil, err
	}
	return w, nil
}
