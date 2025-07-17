package tail

import (
	"slices"

	"github.com/brimdata/super/zbuf"
)

type Op struct {
	parent zbuf.Puller
	limit  int

	batches []zbuf.Batch
	eos     bool
}

func New(parent zbuf.Puller, limit int) *Op {
	//XXX should have a limit check on limit
	return &Op{
		parent: parent,
		limit:  limit,
	}
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	if o.eos {
		// We don't check done here because if we already got EOS,
		// we don't propagate done.
		o.batches = nil
		o.eos = false
		return nil, nil
	}
	if done {
		o.batches = nil
		o.eos = false
		return o.parent.Pull(true)
	}
	if len(o.batches) == 0 {
		batches, err := o.tail()
		if err != nil || len(batches) == 0 {
			return nil, err
		}
		o.batches = batches
	}
	batch := o.batches[0]
	o.batches = o.batches[1:]
	if len(o.batches) == 0 {
		o.eos = true
	}
	return batch, nil
}

// tail pulls from o.parent until EOS and returns batches containing the
// last o.limit values.
func (o *Op) tail() ([]zbuf.Batch, error) {
	var batches []zbuf.Batch
	var n int
	for {
		batch, err := o.parent.Pull(false)
		if err != nil {
			return nil, err
		}
		if batch == nil {
			break
		}
		batches = append(batches, batch)
		n += len(batch.Values())
		for len(batches) > 0 && n-len(batches[0].Values()) >= o.limit {
			// We have enough values without batches[0] so drop it.
			n -= len(batches[0].Values())
			batches[0].Unref()
			batches = slices.Delete(batches, 0, 1)
		}
	}
	if n > o.limit {
		// We have too many values so remove some from batches[0].
		vals := batches[0].Values()[n-o.limit:]
		batches[0] = zbuf.NewBatch(vals)
	}
	return batches, nil
}
