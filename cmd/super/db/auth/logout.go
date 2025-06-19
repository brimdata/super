package auth

import (
	"errors"
	"flag"
	"fmt"

	"github.com/brimdata/super/pkg/charm"
)

var Logout = &charm.Spec{
	Name:  "logout",
	Usage: "auth logout",
	Short: "remove saved credentials for a Zed lake service",
	Long:  ``,
	New:   NewLogoutCommand,
}

type LogoutCommand struct {
	*Command
}

func NewLogoutCommand(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	return &LogoutCommand{Command: parent.(*Command)}, nil
}

func (c *LogoutCommand) Run(args []string) error {
	_, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	if _, err := c.LakeFlags.Connection(); err != nil {
		// The Connection call here is to verify we're operating on a remote lake.
		return err
	}
	if len(args) > 0 {
		return errors.New("logout command takes no arguments")
	}
	if err := c.LakeFlags.AuthStore().RemoveTokens(c.LakeFlags.DB); err != nil {
		return fmt.Errorf("failed to save credentials file: %w", err)
	}
	return nil
}
