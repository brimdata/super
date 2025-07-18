package head

import (
	"github.com/brimdata/super/zbuf"
)

type Op struct {
	parent       zbuf.Puller
	limit, count int
}

func New(parent zbuf.Puller, limit int) *Op {
	return &Op{
		parent: parent,
		limit:  limit,
	}
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	if o.count >= o.limit {
		// If we are at limit we already sent a done upstream,
		// so for either sense of the done flag, we return EOS
		// and reset our state.
		o.count = 0
		return nil, nil
	}
	if done {
		b, err := o.parent.Pull(true)
		if err != nil {
			return nil, err
		}
		if b != nil {
			panic("non-nil done batch")
		}
		o.count = 0
		return nil, nil
	}
again:
	batch, err := o.parent.Pull(false)
	if batch == nil || err != nil {
		o.count = 0
		return nil, err
	}
	remaining := o.limit - o.count
	if remaining <= 0 {
		batch.Unref()
		goto again
	}
	vals := batch.Values()
	if n := len(vals); n < remaining {
		// This batch has fewer than the needed records.
		// Send them all downstream and update the count.
		o.count += n
		return batch, nil
	}
	// This batch has more than the needed records.
	// Signal to the parent that we are done and set the done
	// flag so any downstream dones will not be erroneously
	// propagated since this path is already done.
	if _, err := o.parent.Pull(true); err != nil {
		return nil, err
	}
	o.count = o.limit
	return zbuf.NewBatch(vals[:remaining]), nil
}
