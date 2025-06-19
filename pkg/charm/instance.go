package charm

import (
	"flag"
	"fmt"
	"sort"
	"strings"
)

var (
	HelpFlag     = "h"
	HelpLongFlag = "help"
	HiddenFlag   = "hidden"
)

// instance represents a command that has been created but not run.
// It's options and defaults may be queried with the options method and
// the command can be run with the run method.
type instance struct {
	spec    *Spec
	command Command
	flags   map[string]*flag.Flag
}

// options returns a formatted slice of strings ready for printing as
// help for this instance of a command.
func (i *instance) options(showHidden bool) []string {
	hidden := flagMap(i.spec.HiddenFlags)
	redacted := flagMap(i.spec.RedactedFlags)
	var body []string
	for _, f := range i.flags {
		name := "-" + f.Name
		if hidden[f.Name] {
			if !showHidden {
				continue
			}
			name = "[" + name + "]"
		}
		line := name + " " + f.Usage
		if f.DefValue != "" && !redacted[f.Name] {
			line = fmt.Sprintf("%s (default \"%s\")", line, f.DefValue)
		}
		body = append(body, line)
	}
	sort.Slice(body, func(i, j int) bool {
		return strings.ToLower(body[i]) < strings.ToLower(body[j])
	})
	return body
}

func parse(spec *Spec, args []string, parent Command, interiorLeaf int) (path, []string, bool, error) {
	var path path
	var help, hidden, usage bool
	flags := flag.NewFlagSet(spec.Name, flag.ContinueOnError)
	flags.BoolVar(&help, HelpFlag, false, "display help")
	flags.BoolVar(&help, HelpLongFlag, false, "display help")
	flags.BoolVar(&hidden, HiddenFlag, false, "show hidden options")
	flags.Usage = func() {
		usage = true
	}
	for {
		cmd, err := spec.New(parent, flags)
		if err != nil {
			return nil, nil, false, err
		}
		var haveLeaf bool
		if interiorLeaf > 0 && spec.InternalLeaf {
			interiorLeaf--
			if interiorLeaf == 0 {
				cmd.(InternalLeaf).SetLeafFlags(flags)
				haveLeaf = true
			}
		}
		component := &instance{
			spec:    spec,
			command: cmd,
		}
		path = append(path, component)
		parent = cmd
		if err := flags.Parse(args); err != nil {
			if usage {
				s := strings.Join(args, " ")
				err = fmt.Errorf("at flag: %q: %w", s, err)
			}
			return path, nil, false, err
		}
		if help {
			return path, nil, hidden, NeedHelp
		}
		rest := flags.Args()
		if len(rest) != 0 {
			spec = component.spec.lookupSub(rest[0])
			if spec != nil {
				if haveLeaf {
					return nil, nil, false, ErrNotLeaf
				}
				// We found a subcommand, so continue building the chain.
				args = rest[1:]
				continue
			}
		}
		return path, rest, false, nil
	}
}

func diff(flags *flag.FlagSet, all map[string]*flag.Flag) map[string]*flag.Flag {
	difference := make(map[string]*flag.Flag)
	flags.VisitAll(func(f *flag.Flag) {
		if _, ok := all[f.Name]; !ok {
			all[f.Name] = f
			difference[f.Name] = f
		}
	})
	return difference
}

func parseHelp(spec *Spec, args []string) (path, error) {
	flags := flag.NewFlagSet(spec.Name, flag.ContinueOnError)
	var b bool
	flags.BoolVar(&b, HelpFlag, false, "display help")
	flags.BoolVar(&b, HelpLongFlag, false, "display help")
	flags.BoolVar(&b, HiddenFlag, false, "show hidden options")
	flags.Usage = func() {}
	var parent Command
	all := make(map[string]*flag.Flag)
	var path path
	for {
		cmd, err := spec.New(parent, flags)
		if err != nil {
			return nil, err
		}
		component := &instance{
			spec:    spec,
			command: cmd,
			flags:   diff(flags, all),
		}
		path = append(path, component)
		parent = cmd
		if err := flags.Parse(args); err != nil {
			return nil, err
		}
		rest := flags.Args()
		if len(rest) != 0 {
			spec = component.spec.lookupSub(rest[0])
			if spec != nil {
				// We found a subcommand, so continue building the chain.
				args = rest[1:]
				continue
			}
		}
		// If this is an interior leaf command with leaf flags, then we display
		// all the leaf flags here since the help is being invoked for this
		// interior command.  When it is invoked for a child, this check won't happen.
		if spec.InternalLeaf {
			var flags flag.FlagSet
			cmd.(InternalLeaf).SetLeafFlags(&flags)
			flags.VisitAll(func(f *flag.Flag) {
				if _, ok := component.flags[f.Name]; ok {
					panic(fmt.Sprintf("duplicate flag -%s for command %s", f.Name, path.pathname()))
				}
				component.flags[f.Name] = f
			})
		}
		return path, nil
	}
}
