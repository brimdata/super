package merge

import (
	"container/heap"
	"context"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
)

// Proc merges multiple upstream Pullers into one downstream Puller.
// If the input streams are ordered according to the configured comparison,
// the output of Merger will have the same order.  Each parent puller is run
// in its own goroutine so that deadlock is avoided when the upstream pullers
// would otherwise block waiting for an adjacent puller to finish but the
// Merger is waiting on the upstream puller.
type Op struct {
	ctx context.Context
	cmp expr.CompareFn

	once sync.Once
	// parents holds all of the upstream pullers and never changes.
	parents []*puller
	// The head-of-line (hol) queue is maintained as a min-heap on cmp of
	// hol.vals[0] (see Less) so that the next Read always returns
	// hol[0].vals[0].
	hol   []*puller
	unref sbuf.Batch
}

var _ sbuf.Puller = (*Op)(nil)
var _ sio.Reader = (*Op)(nil)

func New(ctx context.Context, parents []sbuf.Puller, cmp expr.CompareFn) *Op {
	pullers := make([]*puller, 0, len(parents))
	for _, p := range parents {
		pullers = append(pullers, newPuller(ctx, p))
	}
	return &Op{
		ctx:     ctx,
		cmp:     cmp,
		parents: pullers,
	}
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
	var err error
	o.once.Do(func() { err = o.run() })
	if err != nil {
		return nil, err
	}
	if o.unref != nil {
		o.unref.Unref()
		o.unref = nil
	}
	if done {
		return nil, o.propagateDone()
	}
	if o.Len() == 0 {
		// No more batches in head of line.  So, let's resume
		// everything and return an EOS.
		return nil, o.start()
	}
	min := heap.Pop(o).(*puller)
	if o.Len() == 0 || o.cmp(min.vals[len(min.vals)-1], o.hol[0].vals[0]) <= 0 {
		// Either min is the only upstreams or min's last value is less
		// than or equal to the next upstream's first value.  Either
		// way, it's safe to return min's remaining values as a batch.
		batch := min.batch
		if len(min.vals) < len(batch.Values()) {
			batch = sbuf.NewArray(min.vals)
		}
		ok, err := min.replenish()
		if err != nil {
			return nil, err
		}
		if ok {
			heap.Push(o, min)
		}
		return batch, nil
	}
	heap.Push(o, min)
	return sbuf.NewPuller(o).Pull(false)
}

func (o *Op) Read() (*super.Value, error) {
	if o.unref != nil {
		o.unref.Unref()
		o.unref = nil
	}
	if o.Len() == 0 {
		return nil, nil
	}
	u := o.hol[0]
	val := &u.vals[0]
	u.vals = u.vals[1:]
	if len(u.vals) == 0 {
		// Need to unref on next call to Read (or Pull) so keep this around.
		o.unref = u.batch
		ok, err := u.replenish()
		if err != nil {
			return nil, err
		}
		if !ok {
			heap.Pop(o)
			return val, nil
		}
	}
	heap.Fix(o, 0)
	return val, nil
}

func (o *Op) run() error {
	// Start up all the goroutines before initializing the heap.
	// If we do one at a time, there is a deadlock for an upstream
	// split because the split waits for Pulls to arrive before
	// responding.
	for _, parent := range o.parents {
		go parent.run()
	}
	return o.start()
}

// start replenishes each parent's head-of-line batch either at initialization
// or after an EOS and intializes the blocked table with the status of
// each parent, e.g., a parent may be immediately blocked because it has
// no data at (re)start and should not be re-entered into the HOL queue.
func (o *Op) start() error {
	o.hol = o.hol[:0]
	for _, parent := range o.parents {
		parent.blocked = false
		ok, err := parent.replenish()
		if err != nil {
			return err
		}
		if ok {
			heap.Push(o, parent)
		}
	}
	heap.Init(o)
	return nil
}

func (o *Op) propagateDone() error {
	// For everything in the HOL queue (i.e., not already blocked),
	// propagate a done and read until EOS.  This will result in
	// all parents at EOS and blocked; then we can resume everything
	// together.
	for len(o.hol) > 0 {
		m := o.Pop().(*puller)
		select {
		case m.doneCh <- struct{}{}:
			if m.batch != nil {
				m.batch.Unref()
				m.batch = nil
			}
		case <-m.ctx.Done():
			return m.ctx.Err()
		}
	}
	// Now the heap is empty and all pullers are at EOS.
	// Unblock and initialize them so we can resume on the next bill.
	return o.start()
}

func (o *Op) Len() int { return len(o.hol) }

func (o *Op) Less(i, j int) bool {
	return o.cmp(o.hol[i].vals[0], o.hol[j].vals[0]) < 0
}

func (o *Op) Swap(i, j int) { o.hol[i], o.hol[j] = o.hol[j], o.hol[i] }

func (o *Op) Push(x any) { o.hol = append(o.hol, x.(*puller)) }

func (o *Op) Pop() any {
	x := o.hol[len(o.hol)-1]
	o.hol = o.hol[:len(o.hol)-1]
	return x
}

type puller struct {
	sbuf.Puller
	ctx      context.Context
	resultCh chan op.Result
	doneCh   chan struct{}
	batch    sbuf.Batch
	vals     []super.Value
	// Used only by Proc
	blocked bool
}

func newPuller(ctx context.Context, parent sbuf.Puller) *puller {
	return &puller{
		Puller:   op.NewCatcher(parent),
		ctx:      ctx,
		resultCh: make(chan op.Result),
		doneCh:   make(chan struct{}),
	}
}

func (p *puller) run() {
	for {
		batch, err := p.Pull(false)
		select {
		case p.resultCh <- op.Result{Batch: batch, Err: err}:
			if err != nil {
				//XXX
				return
			}
		case <-p.doneCh:
			if batch != nil {
				batch.Unref()
			}
			// Drop the pending batch and initiate a done...
			batch, _ := p.Pull(true) // do something with err
			if batch != nil {
				panic("non-nil done batch")
			}
		case <-p.ctx.Done():
			return
		}
	}
}

// replenish tries to receive the next batch.  It returns false when EOS
// is encountered and its goroutine will then block until resumed or
// canceled.
func (p *puller) replenish() (bool, error) {
	select {
	case r := <-p.resultCh:
		if r.Err != nil {
			return false, r.Err
		}
		p.batch = r.Batch
		if p.batch != nil {
			p.vals = p.batch.Values()
			return true, nil
		}
		p.blocked = true
		return false, nil
	case <-p.ctx.Done():
		return false, p.ctx.Err()
	}
}
