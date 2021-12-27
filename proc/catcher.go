package proc

import (
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zqe"
)

// Catcher wraps an Interface with a Pull method that recovers panics
// and turns them into errors.  It should be wrapped around the output puller
// of a flowgraph and the top-level puller of any goroutine created inside
// of a flowgraph.
type Catcher struct {
	parent Interface
}

func NewCatcher(parent Interface) *Catcher {
	return &Catcher{parent}
}

func (c *Catcher) Pull() (b zbuf.Batch, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = zqe.RecoverError(r)
		}
	}()
	return c.parent.Pull()
}

func (c *Catcher) Done() {
	c.parent.Done()
}
