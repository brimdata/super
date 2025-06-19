package auth

import (
	"errors"
	"flag"
	"fmt"

	"github.com/brimdata/super/api/client/auth0"
	"github.com/brimdata/super/pkg/charm"
)

var Store = &charm.Spec{
	Name:   "store",
	Usage:  "auth store",
	Short:  "store raw tokens",
	Long:   ``,
	New:    NewStore,
	Hidden: true,
}

type StoreCommand struct {
	*Command

	accessToken string
}

func NewStore(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &StoreCommand{Command: parent.(*Command)}
	f.StringVar(&c.accessToken, "access", "", "raw access token as string")
	return c, nil
}

func (c *StoreCommand) Run(args []string) error {
	_, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) > 0 {
		return errors.New("store command takes no arguments")
	}
	if _, err := c.LakeFlags.Connection(); err != nil {
		// The Connection call here is to verify we're operating on a remote database.
		return err
	}
	store := c.LakeFlags.AuthStore()
	tokens, err := store.Tokens(c.LakeFlags.DB)
	if err != nil {
		return fmt.Errorf("failed to load authentication store: %w", err)
	}
	if tokens == nil {
		tokens = &auth0.Tokens{}
	}
	tokens.Access = c.accessToken
	if err := store.SetTokens(c.LakeFlags.DB, *tokens); err != nil {
		return fmt.Errorf("failed to update authentication: %w", err)
	}
	return nil
}
