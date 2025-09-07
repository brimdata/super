package vacuum

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/brimdata/super/cli/poolflags"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/plural"
)

var spec = &charm.Spec{
	Name:  "vacuum",
	Usage: "vacuum [options]",
	Short: "clear space by removing unreferenced objects",
	Long: `
"super db vacuum" clears up space in a pool by removing objects that are not visible 
from a pool's branch or commit.

DANGER ZONE:
Running this command will permanently delete objects referenced in 
previous commits causing missing data when time traveling to those commits.
`,
	New: New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
	poolFlags poolflags.Flags
	dryrun    bool
	force     bool
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	c.poolFlags.SetFlags(f)
	f.BoolVar(&c.dryrun, "dryrun", false, "vacuum without deleting objects")
	f.BoolVar(&c.force, "f", false, "do not prompt for confirmation")
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	at, err := c.poolFlags.HEAD()
	if err != nil {
		return err
	}
	db, err := c.DBFlags.Open(ctx)
	if err != nil {
		return err
	}
	verb := "would vacuum"
	if !c.dryrun {
		verb = "vacuumed"
		if err := c.confirm(at.String()); err != nil {
			return err
		}
	}
	oids, err := db.Vacuum(ctx, at.Pool, at.Branch, c.dryrun)
	if err != nil {
		return err
	}
	if !c.DBFlags.Quiet {
		fmt.Printf("%s %d object%s\n", verb, len(oids), plural.Slice(oids, "s"))
	}
	return nil
}

func (c *Command) confirm(name string) error {
	if c.force {
		return nil
	}
	fmt.Printf("Are you sure you want to vacuum data objects from %q? There is no going back... [y|n]\n", name)
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		return err
	}
	input = strings.ToLower(input)
	if input == "y" || input == "yes" {
		return nil
	}
	return errors.New("operation canceled")
}
