package branches

import (
	"github.com/brimdata/super/pkg/nano"
	"github.com/segmentio/ksuid"
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
