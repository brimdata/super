package branches

import (
	"github.com/segmentio/ksuid"
	"github.com/superdb/super/pkg/nano"
)

type Config struct {
	Ts     nano.Ts     `super:"ts"`
	Name   string      `super:"name"`
	Commit ksuid.KSUID `super:"commit"`

	// audit info
}

func NewConfig(name string, commit ksuid.KSUID) *Config {
	return &Config{
		Ts:     nano.Now(),
		Name:   name,
		Commit: commit,
	}
}

func (c *Config) Key() string {
	return c.Name
}
