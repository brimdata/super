package op

import (
	"context"
	"sync"

	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type Combine struct {
	ctx context.Context

	nblocked int
	once     sync.Once
	parents  []*combineParent
	resultCh chan result
}

func NewCombine(ctx context.Context, parents []vector.Puller) *Combine {
	resultCh := make(chan result, len(parents))
	var combineParents []*combineParent
	for _, p := range parents {
		combineParents = append(combineParents, &combineParent{
			ctx:      ctx,
			parent:   p,
			resultCh: resultCh,
			doneCh:   make(chan struct{}),
			resumeCh: make(chan struct{}),
		})
	}
	return &Combine{
		ctx:      ctx,
		parents:  combineParents,
		resultCh: resultCh,
	}
}

func (c *Combine) Pull(done bool) (vector.Any, error) {
	c.once.Do(func() {
		for _, p := range c.parents {
			go p.run()
		}
	})
	if done {
		// Send done upstream.  Parents waiting on resumeCh will ignore
		// this.  All other parents will transition to waiting on
		// resumeCh.
		var group errgroup.Group
		for _, p := range c.parents {
			// We use a goroutine here because sending to parents[i].doneCh
			// can block until we've sent to parents[i+1].doneCh, as with
			// "fork (=> count() => pass) | head".
			group.Go(func() error {
				return c.signal(p.doneCh)
			})
		}
		if err := group.Wait(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if c.nblocked == len(c.parents) {
		if err := c.resumeParents(); err != nil {
			return nil, err
		}
	}
	for {
		select {
		case r := <-c.resultCh:
			if r.vector != nil || r.err != nil {
				return r.vector, r.err
			}
			// EOS means the sending parent is now blocked.
			c.nblocked++
			if c.nblocked == len(c.parents) {
				// All parents are blocked.  Return EOS.
				return nil, nil
			}
		case <-c.ctx.Done():
			return nil, c.ctx.Err()
		}
	}
}

func (c *Combine) resumeParents() error {
	for _, p := range c.parents {
		if err := c.signal(p.resumeCh); err != nil {
			return err
		}
	}
	c.nblocked = 0
	return nil
}

func (c *Combine) signal(ch chan<- struct{}) error {
	select {
	case ch <- struct{}{}:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

type combineParent struct {
	ctx      context.Context
	parent   vector.Puller
	resultCh chan result
	doneCh   chan struct{}
	resumeCh chan struct{}
}

func (c *combineParent) run() {
	for {
		vec, err := c.parent.Pull(false)
	Select:
		select {
		case c.resultCh <- result{vec, err}:
			if vec == nil && err == nil {
				// EOS blocks us.
				if !c.waitForResume() {
					return
				}
			}
		case <-c.doneCh:
			if vec == nil && err == nil {
				// EOS so don't send done upstream.  If we do,
				// we'll skip the next platoon.
				if !c.waitForResume() {
					return
				}
				continue
			}
			vec, err = c.parent.Pull(true)
			if !c.waitForResume() {
				return
			}
			if err != nil {
				// Send err downstream.
				goto Select
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *combineParent) waitForResume() bool {
	for {
		select {
		case <-c.doneCh:
			// Ignore done while waiting for resume.
		case <-c.resumeCh:
			return true
		case <-c.ctx.Done():
			return false
		}
	}
}
