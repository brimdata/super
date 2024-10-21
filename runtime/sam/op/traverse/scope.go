package traverse

import (
	"context"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/zbuf"
)

type Scope struct {
	ctx         context.Context
	parent      zbuf.Puller
	parentEOSCh chan struct{}
	enter       *Enter
	subgraph    zbuf.Puller
	once        sync.Once
	resultCh    chan op.Result
	exitDoneCh  chan struct{}
	subDoneCh   chan struct{}
}

func newScope(ctx context.Context, parent zbuf.Puller, names []string, exprs []expr.Evaluator) *Scope {
	return &Scope{
		ctx:    ctx,
		parent: parent,
		// Buffered so we can send without blocking before we send EOS
		// to resultCh.
		parentEOSCh: make(chan struct{}, 1),
		enter:       NewEnter(names, exprs),
		resultCh:    make(chan op.Result),
		exitDoneCh:  make(chan struct{}),
		subDoneCh:   make(chan struct{}),
	}
}

func (s *Scope) NewExit(subgraph zbuf.Puller) *Exit {
	s.subgraph = subgraph
	return NewExit(s, len(s.enter.exprs))
}

// Pull is called by the scoped subgraph.
// Parent's batch will already be scoped by Over or Into.
func (s *Scope) Pull(done bool) (zbuf.Batch, error) {
	s.once.Do(func() { go s.run() })
	// Done can happen in two ways with a scope.
	// 1) The output of the scope can be done, e.g., over => (sub) | head
	// 2) The subgraph is done, e.g., over => (sub | head)
	// In case 2, the subgraph is already drained and ready for the next batch.
	if done {
		select {
		case s.subDoneCh <- struct{}{}:
		case <-s.ctx.Done():
			return nil, s.ctx.Err()
		}
	}
	select {
	case r := <-s.resultCh:
		return r.Batch, r.Err
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *Scope) run() {
	for {
		batch, err := s.parent.Pull(false)
		if batch == nil || err != nil {
			select {
			// We send to s.parentEOSCh before s.resultCh to ensure
			// that s.parentEOSCh is ready before the subgraph's
			// Pull returns (in Exit.pullPlatoon).
			case s.parentEOSCh <- struct{}{}:
			case <-s.ctx.Done():
				return
			}
			if ok := s.sendEOS(err); !ok {
				return
			}
		} else if ok := s.sendBatch(batch); !ok {
			return
		}
	}
}

func (s *Scope) sendBatch(b zbuf.Batch) bool {
	select {
	case s.resultCh <- op.Result{Batch: b}:
		if b != nil {
			return s.sendEOS(nil)
		}
		return true
	case <-s.exitDoneCh:
		// If we get a done while trying to send the next batch,
		// we propagate the done to the scope's parent and
		// an EOS since the exit will drain the current platoon
		// to EOS after sending the done.
		if b != nil {
			b.Unref()
		}
		b, err := s.parent.Pull(true)
		if b != nil {
			panic("non-nill done batch")
		}
		return s.sendEOS(err)
	case <-s.subDoneCh:
		// If we get a done from the subgraoh while trying to send
		// the next batch, we shield this done from the scope's parent and
		// send an EOS will terminate the current platoon adhering
		// to the done protocol.
		if b != nil {
			b.Unref()
		}
		return s.sendEOS(nil)
	case <-s.ctx.Done():
		return false
	}
}

func (s *Scope) sendEOS(err error) bool {
again:
	select {
	case s.resultCh <- op.Result{Err: err}:
		return true
	case <-s.exitDoneCh:
		// If we get a done while trying to send an EOS,
		// we'll propagate done to the parent and loop
		// around to send the EOS for the done.
		b, pullErr := s.parent.Pull(true)
		if b != nil {
			panic("non-nill done batch")
		}
		if err == nil {
			err = pullErr
		}
		goto again
	case <-s.subDoneCh:
		// Ignore an internal done from the subgraph as the EOS
		// that's already on the way will ack it.
		goto again
	case <-s.ctx.Done():
		return false
	}
}

type Enter struct {
	names []string
	exprs []expr.Evaluator
}

func NewEnter(names []string, exprs []expr.Evaluator) *Enter {
	return &Enter{
		names: names,
		exprs: exprs,
	}
}

func (e *Enter) addLocals(batch zbuf.Batch, this super.Value) zbuf.Batch {
	inner := newScopedBatch(batch, len(e.exprs))
	for _, expr := range e.exprs {
		// Note that we add a var to the frame on each Eval call
		// since subsequent expressions can refer to results from
		// previous expressions.  Also, we push any val including
		// errors and missing as we want to propagate such conditions
		// into the sub-graph to ease debuging. In fact, the subgraph
		// can act accordingly in response to errors and missing.
		val := expr.Eval(inner, this)
		inner.push(val.Copy())
	}
	return inner
}

type Exit struct {
	scope   *Scope
	nvar    int
	platoon []zbuf.Batch
}

var _ zbuf.Puller = (*Exit)(nil)

func NewExit(scope *Scope, nvar int) *Exit {
	return &Exit{
		scope: scope,
		nvar:  nvar,
	}
}

func (e *Exit) Pull(done bool) (zbuf.Batch, error) {
	if done {
		// Propagate the done to the enter puller then drain
		// the next platoon from the subgraoh.
		select {
		case e.scope.exitDoneCh <- struct{}{}:
		case <-e.scope.ctx.Done():
			return nil, e.scope.ctx.Err()
		}
		err := e.pullPlatoon()
		if err != nil {
			return nil, err
		}
		//XXX unref
		e.platoon = e.platoon[:0]
		return nil, nil
	}
	if len(e.platoon) == 0 {
		if err := e.pullPlatoon(); err != nil {
			return nil, err
		}
		if len(e.platoon) == 0 {
			return nil, nil
		}
	}
	batch := e.platoon[0]
	e.platoon = e.platoon[1:]
	return newExitScope(batch, e.nvar), nil
}

func (e *Exit) pullPlatoon() error {
	for {
		batch, err := e.scope.subgraph.Pull(false)
		if err != nil {
			//XXX unref
			e.platoon = e.platoon[:0]
			return err
		}
		if batch == nil {
			if len(e.platoon) > 0 {
				return nil
			}
			select {
			case <-e.scope.parentEOSCh:
				return nil
			default:
				// We got an empty platoon because the subgraph
				// filtered its input, not because it received
				// consecutive EOSes, so pull the next platoon.
				continue
			}
		}
		e.platoon = append(e.platoon, batch)
	}
}

type scope struct {
	zbuf.Batch
	vars []super.Value
}

var _ zbuf.Batch = (*scope)(nil)

func newScopedBatch(batch zbuf.Batch, nvar int) *scope {
	vars := batch.Vars()
	if len(vars) != 0 {
		// XXX for now we just copy the slice.  we can be
		// more sophisticated later.
		newvars := make([]super.Value, len(vars), len(vars)+nvar)
		copy(newvars, vars)
		vars = newvars
	}
	return &scope{
		Batch: batch,
		vars:  vars,
	}
}

func (s *scope) Vars() []super.Value {
	return s.vars
}

func (s *scope) push(val super.Value) {
	s.vars = append(s.vars, val)
}

type exitScope struct {
	zbuf.Batch
	vars []super.Value
}

var _ zbuf.Batch = (*exitScope)(nil)

func newExitScope(batch zbuf.Batch, nvar int) *exitScope {
	vars := batch.Vars()
	if nvar > len(vars) {
		nvar = len(vars)
	}
	vars = vars[:len(vars)-nvar]
	return &exitScope{
		Batch: batch,
		vars:  vars,
	}
}

func (s *exitScope) Vars() []super.Value {
	return s.vars
}
