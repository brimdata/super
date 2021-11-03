package convert

import (
	"errors"
	"flag"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/cli/inputflags"
	zedindex "github.com/brimdata/zed/cmd/zed/index"
	"github.com/brimdata/zed/field"
	"github.com/brimdata/zed/index"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/anyio"
)

var Convert = &charm.Spec{
	Name:  "convert",
	Usage: "convert [-f frametresh] [ -o file ] -k field[,field,...] file",
	Short: "generate a Zed index file from one or more zng files",
	Long: `
The convert command generates a Zed index containing keys and optional values
from the input file.  The required flag -k specifies one or more zng record
field names that comprise the index search keys, in precedence order.
The keys must be pre-sorted in ascending order with
respect to the stream of zng records; otherwise the index will not work correctly.
The input records are all copied to the base layer of the output index, as is,
so any information stored alongside the keys (e.g., pre-computed aggregations).
It is an error if the key or value fields are not of uniform type.`,
	New: newCommand,
}

func init() {
	zedindex.Cmd.Add(Convert)
}

type Command struct {
	*zedindex.Command
	frameThresh int
	order       string
	outputFile  string
	keys        string
	inputFlags  inputflags.Flags
}

func newCommand(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*zedindex.Command)}
	f.IntVar(&c.frameThresh, "f", 32*1024, "minimum frame size used in Zed index file")
	f.StringVar(&c.order, "order", "asc", "specify data in ascending (asc) or descending (desc) order")
	f.StringVar(&c.outputFile, "o", "index.zng", "name of index output file")
	f.StringVar(&c.keys, "k", "", "comma-separated list of field names for keys")
	c.inputFlags.SetFlags(f, true)

	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.inputFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if c.keys == "" {
		return errors.New("must specify at least one key field with -k")
	}
	//XXX no reason to limit this
	if len(args) != 1 {
		return errors.New("must specify a single zng input file containing keys and optional values")
	}
	path := args[0]
	if path == "-" {
		path = "stdio:stdin"
	}
	zctx := zed.NewContext()
	local := storage.NewLocalEngine()
	file, err := anyio.Open(ctx, zctx, local, path, c.inputFlags.Options())
	if err != nil {
		return err
	}
	o, err := order.Parse(c.order)
	if err != nil {
		return err
	}
	defer file.Close()
	writer, err := index.NewWriter(zctx, local, c.outputFile, field.DottedList(c.keys),
		index.FrameThresh(c.frameThresh),
		index.Order(o),
	)
	if err != nil {
		return err
	}
	if err := zio.Copy(writer, zio.Reader(file)); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}
