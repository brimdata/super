package auth

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/brimdata/zed/pkg/charm"
)

var Method = &charm.Spec{
	Name:  "method",
	Usage: "auth method",
	Short: "display authentication method supported by Zed lake service",
	Long:  ``,
	New:   NewMethod,
}

type MethodCommand struct {
	*Command
}

func NewMethod(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	return &MethodCommand{Command: parent.(*Command)}, nil
}

func (c *MethodCommand) Run(args []string) error {
	ctx, cleanup, err := c.lake.Root().Init()
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) > 0 {
		return errors.New("method command takes no arguments")
	}
	conn, err := c.lake.Connection()
	if err != nil {
		return err
	}
	res, err := conn.AuthMethod(ctx)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(res, "", "\t")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
