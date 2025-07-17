package skip

import (
	"github.com/brimdata/super/zbuf"
)

type Op struct {
	parent zbuf.Puller
	offset int
	count  int
}

func New(parent zbuf.Puller, offset int) *Op {
	return &Op{
		parent: parent,
		offset: offset,
	}
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	for {
		batch, err := o.parent.Pull(done)
		if batch == nil || err != nil {
			o.count = 0
			return nil, err
		}
		if o.count >= o.offset {
			return batch, nil
		}
		vals := batch.Values()
		if remaining := o.offset - o.count; remaining < len(vals) {
			o.count = o.offset
			return zbuf.NewBatch(vals[remaining:]), nil
		}
		o.count += len(vals)
	}
}
