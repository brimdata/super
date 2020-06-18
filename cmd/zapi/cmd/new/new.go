package new

import (
	"errors"
	"flag"
	"fmt"
	"strconv"

	"github.com/alecthomas/units"
	"github.com/brimsec/zq/archive"
	"github.com/brimsec/zq/cmd/zapi/cmd"
	"github.com/brimsec/zq/zqd/api"
	"github.com/brimsec/zq/zqd/storage"
	"github.com/mccanne/charm"
)

var NewSpec = &charm.Spec{
	Name:  "new",
	Usage: "new [spacename]",
	Short: "create a new space",
	Long: `The new command takes a single argument and creates a new, empty space
named as specified.`,
	New: New,
}

func init() {
	cmd.CLI.Add(NewSpec)
}

// bytesValue implements flag.Value.
type bytesValue int64

func (b *bytesValue) Set(s string) error {
	bytes, err := units.ParseStrictBytes(s)
	*b = bytesValue(bytes)
	return err
}

func (b bytesValue) String() string {
	return strconv.FormatInt(int64(b), 10)
}

type Command struct {
	*cmd.Command
	kind     storage.Kind
	datapath string
	thresh   bytesValue
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{
		Command: parent.(*cmd.Command),
		kind:    storage.FileStore,
		thresh:  bytesValue(units.Base2Bytes(archive.DefaultLogSizeThreshold)),
	}
	f.Var(&c.kind, "k", "kind of storage for this space")
	f.StringVar(&c.datapath, "d", "", "specific directory for storage data")
	f.Var(&c.thresh, "thresh", "target size of chopped files, as '10MB', '4GiB', etc.")
	return c, nil
}

func (c *Command) Run(args []string) error {
	if len(args) > 1 {
		return errors.New("too many arguments")
	}
	if len(args) != 1 {
		return errors.New("must specify a space name")
	}

	client := c.Client()
	req := api.SpacePostRequest{
		Name:     args[0],
		DataPath: c.datapath,
		Storage: &storage.Config{
			Kind: c.kind,
			Archive: &storage.ArchiveConfig{
				CreateOptions: &storage.ArchiveCreateOptions{
					LogSizeThreshold: (*int64)(&c.thresh),
				},
			},
		},
	}
	if _, err := client.SpacePost(c.Context(), req); err != nil {
		return fmt.Errorf("couldn't create new space %s: %v", req.Name, err)
	}
	fmt.Printf("%s: space created\n", req.Name)
	return nil
}
