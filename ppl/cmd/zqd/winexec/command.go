package winexec

import (
	"flag"

	"github.com/brimdata/zq/pkg/charm"
	"github.com/brimdata/zq/ppl/cmd/zqd/root"
)

func init() {
	root.Zqd.Add(spec)
}

var spec = &charm.Spec{
	Hidden: true,
	Name:   "winexec",
	Usage:  "winexec <command> <command-options...>",
	Short:  "exec helper for Windows",
	Long: `
Executes the given command, terminating all spawned processes on exit.
`,
	New: newWindowsExecutor,
}

func newWindowsExecutor(_ charm.Command, _ *flag.FlagSet) (charm.Command, error) {
	return &windowsExecutor{}, nil
}

type windowsExecutor struct {
}

func (w *windowsExecutor) Run(args []string) error {
	return winexec(args)
}
