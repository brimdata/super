package dbid

import (
	"errors"
	"fmt"
	"strings"
)

type Committish struct {
	Pool   string `super:"pool"`
	Branch string `super:"branch"`
}

func ParseCommittish(committish string) (*Committish, error) {
	if committish == "" {
		return nil, errors.New("empty pool and branch")
	}
	if strings.IndexByte(committish, '\'') >= 0 {
		return nil, errors.New("pool and branch names may not contain single quote characters")
	}
	if i := strings.LastIndexByte(committish, '@'); i > -1 {
		return &Committish{Pool: committish[:i], Branch: committish[i+1:]}, nil
	}
	return &Committish{Pool: committish}, nil
}

var ErrNoPool = errors.New("no pool")

func (c *Committish) FromSpec(meta string) (string, error) {
	if c.Pool == "" {
		return "", ErrNoPool
	}
	var s string
	if _, err := ParseID(c.Branch); err == nil {
		s = fmt.Sprintf("from '%s'@%s", c.Pool, c.Branch)
	} else {
		s = fmt.Sprintf("from '%s'@'%s'", c.Pool, c.Branch)
	}
	if meta != "" {
		s += ":" + meta
	}
	return s, nil
}

func (c *Committish) String() string {
	return fmt.Sprintf("%s@%s", c.Pool, c.Branch)
}

func (c *Committish) IsZero() bool {
	return c.Pool == "" && c.Branch == ""
}
