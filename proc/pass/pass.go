package pass

import (
	"github.com/brimdata/zed/proc"
	"github.com/brimdata/zed/zbuf"
)

type Proc struct {
	parent proc.Interface
}

func New(parent proc.Interface) *Proc {
	return &Proc{parent}
}

func (p *Proc) Pull() (zbuf.Batch, error) {
	return p.parent.Pull()
}

func (p *Proc) Done() {
	p.parent.Done()
}
