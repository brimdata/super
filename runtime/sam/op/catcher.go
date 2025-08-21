package op

import (
	"fmt"
	"runtime/debug"

	"github.com/brimdata/super/sbuf"
)

// Catcher wraps an Interface with a Pull method that recovers panics
// and turns them into errors.  It should be wrapped around the output puller
// of a flowgraph and the top-level puller of any goroutine created inside
// of a flowgraph.
type Catcher struct {
	parent sbuf.Puller
}

func NewCatcher(parent sbuf.Puller) *Catcher {
	return &Catcher{parent}
}

func (c *Catcher) Pull(done bool) (b sbuf.Batch, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %+v\n%s\n", r, debug.Stack())
		}
	}()
	return c.parent.Pull(done)
}
