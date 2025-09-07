package poolflags

import (
	"errors"
	"flag"

	"github.com/brimdata/super/dbid"
)

type Flags struct {
	defaultHead string
}

func (l *Flags) SetFlags(fs *flag.FlagSet) {
	defaultHead, _ := readHead()
	fs.StringVar(&l.defaultHead, "use", defaultHead, "commit to use, i.e., pool, pool@branch, or pool@commit")
}

func (f *Flags) HEAD() (*dbid.Commitish, error) {
	if f.defaultHead == "" {
		return nil, errors.New(`pool and branch are unspecified
(specify with -use flag or "super db use" command)`)
	}
	c, err := dbid.ParseCommitish(f.defaultHead)
	if err != nil {
		return nil, err
	}
	if c.Pool == "" {
		return nil, errors.New("pool unspecified")
	}
	if c.Branch == "" {
		c.Branch = "main"
	}
	return c, nil
}
