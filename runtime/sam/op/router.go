package op

import (
	"context"
	"slices"
	"sync"

	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sbuf"
)

type Selector interface {
	expr.Resetter
	Forward(*Router, sbuf.Batch) bool
}

type Router struct {
	ctx      context.Context
	parent   sbuf.Puller
	selector Selector
	routes   []*route
	once     sync.Once
}

func NewRouter(ctx context.Context, parent sbuf.Puller) *Router {
	return &Router{
		ctx:    ctx,
		parent: NewCatcher(parent),
	}
}

func (r *Router) Link(s Selector) {
	r.selector = s
}

func (r *Router) AddRoute() sbuf.Puller {
	child := &route{
		router:   r,
		resultCh: make(chan Result),
		doneCh:   make(chan struct{}),
		id:       len(r.routes),
	}
	r.routes = append(r.routes, child)
	return child
}

func (r *Router) run() {
	for {
		if r.blocked() {
			// If everything is blocked, send a done upstream
			// and unblock everything to resume the next platoon.
			batch, err := r.parent.Pull(true)
			if batch != nil {
				panic("non-nil done batch")
			}
			if err != nil {
				if ok := r.sendEOS(err); !ok {
					return
				}
				continue
			}
			r.unblock()
		}
		batch, err := r.parent.Pull(false)
		if err != nil || batch == nil {
			if ok := r.sendEOS(err); !ok {
				return
			}
			r.unblock()
			continue
		}
		// The selectors decides what if any of the batch it
		// wants to send to which down stream procs by calling
		// back the Router.Send() method.  We Unref() the batch here
		// after calling Forward(), so it is up to the selector to Ref()
		// the batch before sending to Send() if it wants to hold on to it.
		if ok := r.selector.Forward(r, batch); !ok {
			return
		}
		batch.Unref()
	}
}

func (r *Router) blocked() bool {
	return !slices.ContainsFunc(r.routes, func(route *route) bool {
		return !route.blocked
	})
}

// Send an EOS to each unblocked route.  On return evertyhing is unblocked
// and everything downstream has been sent an EOS.  If a channel is sending
// done concurrently with the EOS being sent to it, we resolve the done
// with its matching EOS.  Thus, the invariant holds that everything downstream
// has EOS either via Done or batch EOS.  If the Route receives a Pull(done)
// after receiving the EOS, it's done will be captured as soon as we unblock
// all channels.
func (r *Router) sendEOS(err error) bool {
	defer r.selector.Reset()
	// First, we need to send EOS to all non-blocked legs and
	// catch any dones in progress.  This result in all routes
	// being blocked.
	for _, p := range r.routes {
		if p.blocked {
			continue
		}
		// XXX If we get done while trying to send an EOS, we need to
		// ack the done and then send the EOS.  We treat it as if it
		// happened before the EOS being sent.
		select {
		case p.resultCh <- Result{}:
			// This case sends EOS.  If a done arrives first,
			// the rooute won't read from its resultCh and the
			// done case will get captured below.
			p.blocked = true
		case <-p.doneCh:
			// This path was about to be blocked with a done so
			// just mark it blocked now.
			p.blocked = true
		case <-r.ctx.Done():
			return false
		}
	}
	for _, p := range r.routes {
		p.blocked = false
	}
	return true
}

func (r *Router) unblock() {
	for _, p := range r.routes {
		p.blocked = false
	}
}

func (r *Router) Send(p sbuf.Puller, b sbuf.Batch, err error) bool {
	if b == nil {
		panic("EOS sent through router send API")
	}
	to := p.(*route)
	if to.blocked {
		b.Unref()
		return true
	}
	select {
	case to.resultCh <- Result{Batch: b, Err: err}:
		return true
	case <-to.doneCh:
		// If we get a done while trying to write,
		// mark this route blocked and drop the
		// batch being sent.
		b.Unref()
		to.blocked = true
		return true
	case <-r.ctx.Done():
		return false
	}
}

type route struct {
	router   *Router
	resultCh chan Result
	doneCh   chan struct{}
	id       int
	// Used only by Router
	blocked bool
}

func (r *route) Pull(done bool) (sbuf.Batch, error) {
	r.router.once.Do(func() {
		go r.router.run()
	})
	if done {
		select {
		case r.doneCh <- struct{}{}:
			return nil, nil
		case <-r.router.ctx.Done():
			return nil, r.router.ctx.Err()
		}
	}
	select {
	case result := <-r.resultCh:
		return result.Batch, result.Err
	case <-r.router.ctx.Done():
		return nil, r.router.ctx.Err()
	}
}
