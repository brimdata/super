package create

import (
	"errors"
	"flag"

	"github.com/brimdata/zq/cli/inputflags"
	"github.com/brimdata/zq/cmd/microindex/root"
	"github.com/brimdata/zq/expr"
	"github.com/brimdata/zq/field"
	"github.com/brimdata/zq/microindex"
	"github.com/brimdata/zq/pkg/charm"
	"github.com/brimdata/zq/pkg/iosrc"
	"github.com/brimdata/zq/zbuf"
	"github.com/brimdata/zq/zio/detector"
	"github.com/brimdata/zq/zng/resolver"
)

var Create = &charm.Spec{
	Name:  "create",
	Usage: "create [-f frameThresh] [ -o file ] -k field file",
	Short: "create a key-only microindex from a zng file",
	Long: `
The create command generates a key-only microindex file comprising the values from the
input taken from the field specified by -k.  The output index will have a base layer
with search key called "key".
If a key appears more than once, the last value in the input takes precedence.
It is an error if the key fields are not of uniform type.`,
	New: newCommand,
}

func init() {
	root.MicroIndex.Add(Create)
}

type Command struct {
	*root.Command
	frameThresh int
	outputFile  string
	keyField    string
	skip        bool
	inputReady  bool
	inputFlags  inputflags.Flags
}

func newCommand(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{
		Command: parent.(*root.Command),
	}
	f.IntVar(&c.frameThresh, "f", 32*1024, "minimum frame size used in microindex file")
	f.StringVar(&c.outputFile, "o", "index.zng", "name of microindex output file")
	f.StringVar(&c.keyField, "k", "", "field name of search keys")
	f.BoolVar(&c.inputReady, "x", false, "input file is already sorted keys (and optional values)")
	f.BoolVar(&c.skip, "S", false, "skip all records except for the first of each stream")
	c.inputFlags.SetFlags(f)

	return c, nil
}

func (c *Command) Run(args []string) error {
	defer c.Cleanup()
	if err := c.Init(&c.inputFlags); err != nil {
		return err
	}
	if c.keyField == "" {
		return errors.New("must specify at least one key field with -k")
	}
	//XXX no reason to limit this... we will fix this when we refactor
	// the code here to use zql/proc instead for the hash table (after we
	// have spillable group-bys)
	if len(args) != 1 {
		return errors.New("must specify a single zng input file containing the indicated keys")
	}
	path := args[0]
	if path == "-" {
		path = iosrc.Stdin
	}
	zctx := resolver.NewContext()
	file, err := detector.OpenFile(zctx, path, c.inputFlags.Options())
	if err != nil {
		return err
	}
	writer, err := microindex.NewWriter(zctx, c.outputFile, microindex.FrameThresh(c.frameThresh))
	if err != nil {
		return err
	}
	close := true
	defer func() {
		if close {
			writer.Close()
		}
	}()
	reader, err := c.buildTable(zctx, file)
	if err != nil {
		return err
	}
	if err := zbuf.Copy(writer, reader); err != nil {
		return err
	}
	close = false
	return writer.Close()
}

func (c *Command) buildTable(zctx *resolver.Context, reader zbuf.Reader) (*microindex.MemTable, error) {
	readKey := expr.NewDotExpr(field.Dotted(c.keyField))
	table := microindex.NewMemTable(zctx)
	for {
		rec, err := reader.Read()
		if err != nil {
			return nil, err
		}
		if rec == nil {
			break
		}
		k, err := readKey.Eval(rec)
		if err != nil || k.Type == nil {
			// if the key doesn't exist, just skip it
			continue
		}
		if k.Bytes == nil {
			// The key field is unset.  Skip it.  Unless we want to
			// index the notion of something that is unset, this is
			// the right thing to do.
			continue
		}
		if err := table.EnterKey(k); err != nil {
			return nil, err
		}
	}
	return table, nil
}
