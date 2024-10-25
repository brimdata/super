// Package charm is minimilast CLI framework inspired by cobra and urfave/cli.
package charm

import (
	"errors"
	"flag"
)

var (
	NeedHelp   = errors.New("help")
	ErrNoRun   = errors.New("no run method")
	ErrNotLeaf = errors.New("no internal leaf found")
)

type Constructor func(Command, *flag.FlagSet) (Command, error)

type Command interface {
	Run([]string) error
}

// An interior leaf command that implements SetLeafFlags has at least
// some flags that are used by that subcommand but has children that
// don't inherit these flags.
type InternalLeaf interface {
	SetLeafFlags(*flag.FlagSet)
}

type Spec struct {
	Name  string
	Usage string
	Short string
	Long  string
	New   Constructor
	// Hidden hides this command from help.
	Hidden bool
	// Hidden flags (comma-separated) marks these flags as hidden.
	HiddenFlags string
	// Redacted flags (comma-separated) marks these flags as redacted,
	// where a flag is shown (if not hidden) but its default value is hidden,
	// e.g., as is useful for a password flag.
	RedactedFlags string
	// True for commands that have internal leaf flags.
	// We can't infer this by asserting the InternalLeaf interface when
	// command hierarchies embed and export parent command structs to children so
	// we have this flag to override such exportation.
	InternalLeaf bool
	children     []*Spec
	parent       *Spec
}

func (c *Spec) Add(child *Spec) {
	c.children = append(c.children, child)
	child.parent = c
}

func (c *Spec) lookupSub(name string) *Spec {
	for _, child := range c.children {
		if name == child.Name {
			return child
		}
	}
	return nil
}

func (s *Spec) Exec(args []string) error {
	path, rest, showHidden, err := parse(s, args, nil, true)
	if err == ErrNotLeaf {
		path, rest, showHidden, err = parse(s, args, nil, false)
	}
	if err == nil {
		err = path.run(rest)
	}
	if err == NeedHelp {
		path, err := parseHelp(s, args)
		if err != nil {
			return err
		}
		displayHelp(path, showHidden)
		return nil
	}
	return err
}

func NoRun(args []string) error {
	if len(args) == 0 {
		return NeedHelp
	}
	return ErrNoRun
}
